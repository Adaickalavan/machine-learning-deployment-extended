package main

import (
	"encoding/json"
	"image"
	"image/color"
	"log"
	"goconsumer/models"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"gocv.io/x/gocv"
)

var statusColor = color.RGBA{200, 150, 50, 0}
var bkgColor = color.RGBA{255, 255, 255, 0}
var boxColor = color.RGBA{0, 0, 255, 0}

func message(ev *kafka.Message) error {

	//Read message into `topicMsg` struct
	doc := &topicMsg{}
	err := json.Unmarshal(ev.Value, doc)
	if err != nil {
		log.Println(err)
		return err
	}

	// Retrieve frame
	log.Printf("%% Message sent %v on %s\n", ev.Timestamp, ev.TopicPartition)
	frame, err := gocv.NewMatFromBytes(doc.Rows, doc.Cols, doc.Type, doc.Mat)
	if err != nil {
		log.Println("Frame:", err)
		return err
	}

	// Clone frame
	frameOut := frame.Clone()

	// Form output image
	for ind := 0; ind < len(modelParams); ind++ {
		mp := modelParams[ind]
		parts := strings.Split(mp.modelName, "_")
		switch parts[0] {
		case "imagenet":
			// Get prediction
			res, err := mp.modelHandler.Get()
			if err == nil {
				mp.pred = res.Class
			}
			// Write prediction to frame
			gocv.PutText(
				&frameOut,
				mp.pred,
				image.Pt(10, ind*20+20),
				gocv.FontHersheyPlain, 1.2,
				bkgColor, 6,
			)
			// Write prediction to frame
			gocv.PutText(
				&frameOut,
				mp.pred,
				image.Pt(10, ind*20+20),
				gocv.FontHersheyPlain, 1.2,
				statusColor, 2,
			)
		case "emonet":
			// Get prediction
			res, err := mp.modelHandler.Get()
			if err != nil {
				break
			}
			// draw a rectangle around each face on the original image,
			// along with text identifying as "Human"
			var rMinY int
			for index, r := range res.Rects {
				gocv.Rectangle(&frameOut, r, boxColor, 2)
				if r.Min.Y-5 < 10 {
					rMinY = 10
				} else {
					rMinY = r.Min.Y
				}
				gocv.PutText(
					&frameOut,
					res.ClassArr[index],
					image.Pt(r.Min.X, rMinY),
					gocv.FontHersheyPlain, 1.2,
					bkgColor, 6,
				)
				gocv.PutText(
					&frameOut,
					res.ClassArr[index],
					image.Pt(r.Min.X, rMinY),
					gocv.FontHersheyPlain, 1.2,
					statusColor, 2,
				)
			}
		default:
			log.Fatal("Model not recognised")
		}

		// Post next frame
		mp.modelHandler.Post(models.Input{Img: frame})
	}

	// Write image to output Kafka queue
	select {
	case videoDisplay <- frameOut:
	default:
	}
	// videoDisplay <- frameOut

	return nil
}

type topicMsg struct {
	Mat      []byte       `json:"mat"`
	Channels int          `json:"channels"`
	Rows     int          `json:"rows"`
	Cols     int          `json:"cols"`
	Type     gocv.MatType `json:"type"`
}
