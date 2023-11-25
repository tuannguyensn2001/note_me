package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"
	"os"
	"regexp"
	"strings"
	"time"
)

type Word struct {
	Query     string     `json:"query" bson:"query"`
	Parent    []string   `json:"parent" bson:"parent"`
	Text      [][]string `json:"text" bson:"text"`
	CreatedAt int64      `json:"created_at" bson:"created_at"`
	UpdatedAt int64      `json:"updated_at" bson:"updated_at"`
}

func main() {

	r := gin.Default()
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(os.Getenv("DB_URL")).SetServerAPIOptions(serverAPI)
	cache, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		panic(err)
	}

	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
	// Send a ping to confirm a successful connection
	if err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"ping", 1}}).Err(); err != nil {
		panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
	r.GET("/word/:word", func(context *gin.Context) {
		word := context.Param("word")

		result, err := handleFindWord(context, word, client, cache)
		if err != nil {
			context.JSON(500, gin.H{
				"message": "find failed",
			})
			return
		}
		context.JSON(200, gin.H{
			"data": result,
		})
	})

	r.POST("/seed", func(ctx *gin.Context) {
		type Payload struct {
			Sentence string `json:"sentence" form:"sentence"`
		}
		var payload Payload
		if err := ctx.ShouldBind(&payload); err != nil {
			ctx.JSON(400, gin.H{
				"message": "invalid payload",
			})
			return
		}
		words := filter(payload.Sentence)
		g, _ := errgroup.WithContext(ctx.Request.Context())
		for _, word := range words {
			word := word
			g.Go(func() error {
				_, err := handleFindWord(ctx, word, client, cache)
				return err
			})
		}
		if err := g.Wait(); err != nil {
			ctx.JSON(500, gin.H{
				"message": "seed failed",
			})
			return
		}

		ctx.JSON(200, gin.H{
			"message": "success",
		})
	})

	log.Info().Msg("server running")
	r.Run(":16000")
}

func handleFindWord(ctx context.Context, word string, client *mongo.Client, cache *badger.DB) (*Word, error) {
	parent := make([]string, 0)
	text := make([][]string, 0)
	c := colly.NewCollector()
	var item Word
	log.Info().Str("word", word).Msg("processing word")
	err := cache.View(func(txn *badger.Txn) error {
		raw, err := txn.Get([]byte(word))
		if err != nil {
			return err
		}
		var rawValue []byte
		raw.Value(func(val []byte) error {
			rawValue = val
			return nil
		})

		if err := json.NewDecoder(bytes.NewReader(rawValue)).Decode(&item); err != nil {
			return err
		}
		return nil
	})
	if err == nil {
		return &item, nil
	}
	err = nil

	err = client.Database("noteme").Collection("words").FindOne(ctx, bson.M{
		"word": word,
	}).Decode(&item)
	if err == nil {
		return &item, nil
	}

	c.OnHTML(".slide_content", func(element *colly.HTMLElement) {
		validElement := !strings.Contains(element.Attr("class"), "hidden")
		if !validElement {
			return
		}
		element.ForEach("div", func(i int, element *colly.HTMLElement) {

			if element.Attr("class") == "bg-grey bold font-large m-top20" {
				parent = append(parent, element.Text)
			} else if element.Attr("class") == "green bold margin25 m-top15" {
				if len(text) != len(parent) || len(parent) == 0 {
					text = append(text, make([]string, 0))
				}
				text[len(text)-1] = append(text[len(text)-1], element.Text)
			}
		})

		item = Word{
			Query:     word,
			Parent:    parent,
			Text:      text,
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		}
		if _, err1 := client.Database("noteme").Collection("words").InsertOne(ctx, &item); err1 != nil {
			err = err1
		}

		go func() {
			err := cache.Update(func(txn *badger.Txn) error {
				var b bytes.Buffer
				if err := json.NewEncoder(&b).Encode(&item); err != nil {
					return err
				}

				if err := txn.Set([]byte(word), b.Bytes()); err != nil {
					return err
				}

				return nil
			})
			if err != nil {
				log.Error().Err(err).Msg("cache fail")
			}
		}()
	})

	c.OnRequest(func(r *colly.Request) {
		//log.Println("Visiting", r.URL)
	})
	c.Visit("https://dict.laban.vn/find?type=1&query=" + word)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	return &item, nil
}

func filter(sentence string) []string {
	re := regexp.MustCompile(`[^a-zA-Z ]`)
	sentence = re.ReplaceAllString(sentence, "")
	sentence = strings.ToLower(sentence)

	// Split the sentence into words
	words := strings.Fields(sentence)
	return words
}
