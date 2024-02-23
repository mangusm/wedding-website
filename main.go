package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type RsvpLastName struct {
	LastName string `form:"lastName"`
	GuestId  string `form:"guestId"`
}

type RsvpSubmit struct {
	GuestIds          []string `form:"guestIds"`
	GuestsAttending   []string `form:"guestsAttending"`
	PlusOnesAttending []string `form:"plusOnesAttending"`
	Notes             string   `form:"notes"`
}

type Guest struct {
	Id               pgtype.Text `db:"id"`
	InvitationId     pgtype.Text `db:"invitation_id"`
	FirstName        pgtype.Text `db:"first_name"`
	LastName         pgtype.Text `db:"last_name"`
	Attending        pgtype.Bool `db:"attending"`
	PlusOneAllowed   pgtype.Bool `db:"plus_one_allowed"`
	PlusOneAttending pgtype.Bool `db:"plus_one_attending"`
	Notes            pgtype.Text `db:"notes"`
	HasRsvpd         pgtype.Bool `db:"has_rsvpd"`
}

type JsonLog struct {
	ClientIp        string
	Timestamp       string
	Method          string
	Path            string
	RequestFormData map[string][]string
	Proto           string
	StatusCode      int
	Latency         time.Duration
	UserAgent       string
	ErrorMessage    string
}

func main() {
	// if this was started by air, then load .env, otherwise expect env vars to be set
	if os.Getenv("ENVIRONMENT") == "air" {
		err := godotenv.Load(".env")
		if err != nil {
			panic(err)
		}
	}

	ctx := context.Background()
	log.Println("Connecting to database...")
	conn, err := pgx.Connect(ctx, os.Getenv("DB_CONNECTION_STRING"))
	if err != nil {
		panic(err)
	}
	defer conn.Close(context.Background())

	genericErrorMessage := "Something went wrong, try again later or get in touch with us directly."
	genericErrorH := gin.H{"Message": genericErrorMessage}

	r := gin.New()
	r.Static("/css", "./static/css")
	r.Static("/assets", "./static/assets")

	// TODO:
	r.StaticFile("/travel", "./static/templates/wip.html")
	r.StaticFile("/registry", "./static/templates/wip.html")
	r.LoadHTMLGlob("static/templates/*.html")

	// JSON logging
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		errorMessage := strings.Replace(param.ErrorMessage, "\n", " ", -1)
		logData := JsonLog{
			ClientIp:        param.ClientIP,
			Timestamp:       param.TimeStamp.Format(time.RFC1123),
			Method:          param.Method,
			Path:            param.Path,
			RequestFormData: param.Request.PostForm,
			Proto:           param.Request.Proto,
			StatusCode:      param.StatusCode,
			Latency:         param.Latency,
			UserAgent:       param.Request.UserAgent(),
			ErrorMessage:    errorMessage,
		}
		jsonData, _ := json.Marshal(logData)
		if err != nil {
			log.Println(fmt.Sprintf("Could not marshal json data: %s", err))
			return fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s|%s\n", param.ClientIP, param.TimeStamp.Format(time.RFC1123), param.Method, param.Path, param.Request.Proto, param.StatusCode, param.Latency, param.Request.UserAgent(), errorMessage)
		}
		return string(jsonData) + "\n"
	}))

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	r.GET("/rsvp", func(c *gin.Context) {
		c.HTML(http.StatusOK, "rsvp.html", gin.H{})
	})

	// This route gets called if duplicate last names are found
	r.POST("/rsvp/findById", func(c *gin.Context) {
		var formData RsvpLastName
		err := c.Bind(&formData)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		guestId := strings.TrimSpace(formData.GuestId)
		row, err := conn.Query(ctx, "select * from guests where id = $1", guestId)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		guest, err := pgx.CollectExactlyOneRow(row, pgx.RowToStructByName[Guest])
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		rows, err := conn.Query(ctx, "select * from guests where invitation_id = $1", guest.InvitationId.String)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		guests, err := pgx.CollectRows(rows, pgx.RowToStructByName[Guest])
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}
		c.HTML(http.StatusOK, "submit.html", gin.H{
			"Guests": guests,
		})
	})

	r.POST("/rsvp/findByLastName", func(c *gin.Context) {
		var formData RsvpLastName
		err := c.Bind(&formData)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		lastName := strings.ToLower(strings.TrimSpace(formData.LastName))

		// Just do nothing if the name is empty
		if len(lastName) == 0 {
			c.Header("HX-Reswap", "none")
			c.Status(http.StatusNoContent)
			return
		}

		rows, err := conn.Query(ctx, "select * from guests where lower(last_name) = $1 order by first_name asc", lastName)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		guests, err := pgx.CollectRows(rows, pgx.RowToStructByName[Guest])
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		// Check if there were more than one invitations sent out for people of the same last name
		invitationIds := make(map[string]bool)
		for _, guest := range guests {
			invitationIds[guest.InvitationId.String] = true
		}

		if len(invitationIds) > 1 {
			c.HTML(http.StatusMultipleChoices, "multiple-invitations.html", gin.H{"Guests": guests})
			return
		} else if len(invitationIds) == 1 {
			// In case there were others on the invitation with different last names, fetch everyone with that invitation ID
			invitationId := guests[0].InvitationId.String
			rows, err := conn.Query(ctx, "select * from guests where invitation_id = $1", invitationId)
			if err != nil {
				c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
				c.Error(err)
				return
			}

			guests, err := pgx.CollectRows(rows, pgx.RowToStructByName[Guest])
			if err != nil {
				c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
				c.Error(err)
				return
			}
			c.HTML(http.StatusOK, "submit.html", gin.H{
				"Guests": guests,
			})
			return
		}
		c.HTML(http.StatusNotFound, "ErrorMessage", gin.H{"Message": "Sorry, we didn't send an inviation to anyone by that name."})
		c.Error(errors.New(fmt.Sprintf("Could not find guest by last name")))
	})

	r.POST("/rsvp/submit", func(c *gin.Context) {
		var formData RsvpSubmit
		err := c.Bind(&formData)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		// The plus one can't attend without the invited guest
		for _, id := range formData.PlusOnesAttending {
			if !slices.Contains(formData.GuestsAttending, id) {
				c.HTML(http.StatusBadRequest, "ErrorMessage", gin.H{"Message": "We're sure your plus one is great and all, but they can't come without you."})
				c.Error(errors.New("Plus one cannot attend by themselves"))
				return
			}
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		for _, guestId := range formData.GuestIds {
			if slices.Contains(formData.GuestsAttending, guestId) {
				plusOneIsAttending := slices.Contains(formData.PlusOnesAttending, guestId)
				_, err := tx.Exec(ctx, "update guests set has_rsvpd = true, attending = true, plus_one_attending = $1, notes = $2 where id = $3", plusOneIsAttending, formData.Notes, guestId)
				if err != nil {
					c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
					c.Error(err)
					return
				}
			} else {
				_, err := tx.Exec(ctx, "update guests set has_rsvpd = true, attending = false, plus_one_attending = false, notes = $1 where id = $2", formData.Notes, guestId)
				if err != nil {
					c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
					c.Error(err)
					return
				}
			}
		}

		err = tx.Commit(ctx)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "ErrorMessage", genericErrorH)
			c.Error(err)
			return
		}

		c.HTML(http.StatusOK, "thankyou.html", gin.H{
			"Guest":   len(formData.GuestsAttending) > 0,
			"PlusOne": len(formData.PlusOnesAttending) > 0,
		})
	})

	r.Run(":" + os.Getenv("PORT"))
}
