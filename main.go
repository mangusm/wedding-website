package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"

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

func main() {
	isDev := os.Getenv("ENVIRONMENT") == "dev"

	programLevel := new(slog.LevelVar) // Info by default
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel, AddSource: isDev})
	slog.SetDefault(slog.New(h))

	cfg, err := pgx.ParseConfig(fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s", os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), os.Getenv("POSTGRES_DB")))
	if err != nil {
		slog.Error(err.Error())
	}

	// if we're developing then load our .env
	if isDev {
		programLevel.Set(slog.LevelDebug)
		err = godotenv.Load(".env")
		if err != nil {
			slog.Error(err.Error())
		}
		cfg, err = pgx.ParseConfig(os.Getenv("DATABASE_URL"))
		if err != nil {
			slog.Error(err.Error())
		}
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.ConnString())
	if err != nil {
		slog.Error(err.Error())
		panic(err)
	}
	defer conn.Close(context.Background())

	genericErrorMessage := "Something went wrong, try again later or get in touch with us directly."
	genericErrorH := gin.H{"Message": genericErrorMessage}

	r := gin.New()
	r.Static("/css", "./static/css")
	r.Static("/assets", "./static/assets")

	// TODO:
	r.StaticFile("/registry", "./static/templates/wip.html")
	r.LoadHTMLGlob("static/templates/*.html")

	// JSON logging
	r.Use(func(c *gin.Context) {
		c.Next()
		slog.Info("handle",
			"method",
			c.Request.Method,
			"path",
			c.Request.URL.Path,
			"status_code",
			c.Writer.Status(),
			"client_ip",
			c.ClientIP(),
			"user_agent",
			c.Request.UserAgent(),
			"post_form",
			c.Request.PostForm,
			"errors",
			c.Errors,
		)
	})

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{})
	})

	r.GET("/travel", func(c *gin.Context) {
		c.HTML(http.StatusOK, "travel.html", gin.H{})
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
		c.HTML(http.StatusNotFound, "ErrorMessage", gin.H{"Message": "Sorry, we didn't send an invitation to anyone by that name."})
		c.Error(fmt.Errorf("could not find guest by last name"))
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
				c.Error(errors.New("plus one cannot attend by themselves"))
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
