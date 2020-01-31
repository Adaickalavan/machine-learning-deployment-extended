package main

import (
	"encoding/json"
	"goproducer/confluentkafkago"
	"image"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"gocv.io/x/gocv"
)

func main() {

	broker := os.Getenv("KAFKAPORT")
	topic := os.Getenv("TOPICNAME")
	frameInterval, err := time.ParseDuration(os.Getenv("FRAMEINTERVAL"))
	if err != nil {
		log.Fatal("Invalid frame interval", err)
	}
	compression := os.Getenv("COMPRESSIONTYPE")

	p, _, err := confluentkafkago.NewProducer(broker, compression)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		// Close the producer
		p.Flush(1000)
		p.Close()
	}()

	fx, _ := strconv.ParseFloat(os.Getenv("IMAGESCALEX"), 64)
	fy, _ := strconv.ParseFloat(os.Getenv("IMAGESCALEY"), 64)
	go readWebcam(p, frameInterval, topic, fx, fy)

	//Block forever
	select {}
}

//Result represents the Kafka queue message format
type topicMsg struct {
	Mat      []byte       `json:"mat"`
	Channels int          `json:"channels"`
	Rows     int          `json:"rows"`
	Cols     int          `json:"cols"`
	Type     gocv.MatType `json:"type"`
}

func getenvint(str string) int {
	i, err := strconv.Atoi(os.Getenv(str))
	if err != nil {
		log.Fatal(err)
	}
	return i
}

func readWebcam(p *kafka.Producer, frameInterval time.Duration, topic string, fx float64, fy float64) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("main.readWebcam():PANICKED AND RESTARTING")
			log.Println("Panic:", r)
			go readWebcam(p, frameInterval, topic, fx, fy)
		}
	}()

	// Capture video from device
	webcam, err := gocv.VideoCaptureDevice(getenvint("VIDEODEVICE"))
	// Capture video from internet stream
	// webcam, err := gocv.OpenVideoCapture(os.Getenv("VIDEOLINK"))
	if err != nil {
		panic("Error in opening webcam: " + err.Error())
	}
	defer webcam.Close()

	// Stream images from RTSP to Kafka message queue
	origframe := gocv.NewMat()
	frame := gocv.NewMat()
	sz := image.Point{}
	for {
		if !webcam.Read(&origframe) {
			panic("Webcam read failure")
		}

		//Downsize frame
		gocv.Resize(origframe, &frame, sz, fx, fy, gocv.InterpolationFlags(1))

		//Form the struct to be sent to Kafka message queue
		doc := topicMsg{
			Mat:      frame.ToBytes(),
			Channels: frame.Channels(),
			Rows:     frame.Rows(),
			Cols:     frame.Cols(),
			Type:     frame.Type(),
		}

		//Prepare message to be sent to Kafka
		docBytes, err := json.Marshal(doc)
		if err != nil {
			log.Println("Json marshalling error. Error:", err.Error())
			continue
		}

		//Send message into Kafka queue
		p.ProduceChannel() <- &kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          docBytes,
			Timestamp:      time.Now(),
		}

		log.Printf("%% Message sent %v\n", time.Now())
		log.Println("row :", frame.Rows(), " col: ", frame.Cols())

		//Wait for xx milliseconds
		time.Sleep(frameInterval)

		//Read delivery report before producing next message
		// <-doneChan
	}
}
