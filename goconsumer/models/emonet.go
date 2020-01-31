package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"io/ioutil"
	"log"
	"net/http"

	"gocv.io/x/gocv"
)

const (
	width  = 48
	height = 48
)

type stdImg [height][width][1]float32

type emonet struct {
	baseHandler
	faceCascade gocv.CascadeClassifier
	minSize     image.Point
	maxSize     image.Point
	sz          image.Point
}

//NewEmonet returns a new handle to specified machine learning model
func NewEmonet(modelurl string, labelurl string) (Handler, error) {

	labels := make(map[int]string)

	// Read-in labels
	dat, err := ioutil.ReadFile(labelurl)
	if err != nil {
		return &emonet{}, errors.New("Failed to read in labelurl. " + err.Error())
	}
	err = json.Unmarshal(dat, &labels)
	if err != nil {
		return &emonet{}, errors.New("Failure in unmarshalling labels. " + err.Error())
	}

	// load classifier to recognize faces
	faceCascade := gocv.NewCascadeClassifier()
	xmlFile := "/src/models/haarcascade_frontalface_default.xml"
	if !faceCascade.Load(xmlFile) {
		log.Println("Error reading cascade file:", xmlFile)
		return &emonet{}, errors.New("Error reading cascade file")
	}

	return &emonet{
		baseHandler: baseHandler{
			labels: labels,
			url:    modelurl,
			chIn:   make(chan Input),
			chOut:  make(chan Output),
		},
		faceCascade: faceCascade,
		minSize:     image.Point{X: width, Y: height},
		maxSize:     image.Point{X: 50000, Y: 50000},
		sz:          image.Point{X: width, Y: height},
	}, nil
}

//Predict classifies input images
func (emn *emonet) Predict() {
	var resBody responseBodyTensor
	defer func() {
		if r := recover(); r != nil {
			log.Println("models.*emonet.Predict():PANICKED AND RESTARTING")
			log.Println("Panic:", r)
			go emn.Predict()
		}
	}()

	//Write initial prediction into shared output channel
	emn.chOut <- Output{Class: "Nothing"}

	for elem := range emn.chIn {
		img := elem.Img
		// imgGray := gocv.NewMat()

		// Convert img to gray scale
		// gocv.CvtColor(img, &imgGray, gocv.ColorConversionCode(6))
		gocv.CvtColor(img, &img, gocv.ColorConversionCode(6))

		// Detect faces
		// rects := emn.faceCascade.DetectMultiScale(img)
		// rects := emn.faceCascade.DetectMultiScaleWithParams(imgGray, 1.3, 5, 0, emn.minSize, emn.maxSize)
		rects := emn.faceCascade.DetectMultiScaleWithParams(img, 1.3, 5, 0, emn.minSize, emn.maxSize)
		log.Println("found", len(rects), "faces")

		classArr := []string{}
		facesArr := []stdImg{}
		rectArr := []image.Rectangle{}
		for _, r := range rects {
			//Crop and resize image
			imgFace := img.Region(r)
			imgFaceDet := imgFace.Clone()
			// gocv.CvtColor(imgFaceDet, &imgFaceDet, gocv.ColorConversionCode(6))
			gocv.Resize(imgFaceDet, &imgFaceDet, emn.sz, 0, 0, 1)

			// Convert OpenCV Mat to MatTypeCV32F
			imgFaceDet.ConvertTo(&imgFaceDet, gocv.MatType(5))
			// Normalize image
			imgFaceDet.DivideFloat(float32(255))
			// Convert MatTypeCV32F to slice of float32
			imgSlice, err := imgFaceDet.DataPtrFloat32()
			if err != nil {
				log.Println("Error in DataPtrFloat32 of OpenCV Mat. ", err)
				continue
			}

			// Convert slice into multidimensional array
			var imgTensor stdImg
			for row := 0; row < height; row++ {
				for col := 0; col < width; col++ {
					imgTensor[row][col][0] = imgSlice[row*width+col]
				}
			}

			rectArr = append(rectArr, r)
			facesArr = append(facesArr, imgTensor)
		}

		if len(facesArr) == 0 {
			continue
		}

		//Prepare request message
		inference := inferTensor{
			InstancesTensor: facesArr,
		}

		//Query the machine learning model
		reqBody, err := json.Marshal(inference)
		if err != nil {
			log.Println("Error in Marshal: ", err)
			continue
		}
		req, err := http.NewRequest("POST", emn.url, bytes.NewBuffer(reqBody))
		if err != nil {
			log.Println("Error in NewRequest: ", err)
			continue
		}
		req.Header.Add("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Println("Error in DefaultClient: ", err)
			continue
		}
		defer res.Body.Close()

		// body, err := ioutil.ReadAll(res.Body)
		// log.Println("++++++++++\n", string(body), "\n++++++++++\n")

		//Process response from machine learning model
		decoder := json.NewDecoder(res.Body)
		if err := decoder.Decode(&resBody); err != nil {
			log.Println("Error in Decode: ", err)
			continue
		}

		for ii := range facesArr {
			probabilities := resBody.PredictionsTensor[ii]
			probMax := float32(0)
			indexMax := 6
			for jj, prob := range probabilities {
				if prob > probMax {
					probMax = prob
					indexMax = jj
				}
			}
			pred, _ := emn.labels[indexMax]
			classArr = append(classArr, pred)
		}

		//Write prediction into shared output channel
		emn.chOut <- Output{Rects: rectArr, ClassArr: classArr}
	}
}

type inferTensor struct {
	InstancesTensor []stdImg `json:"instances"`
}

type responseBodyTensor struct {
	PredictionsTensor [][]float32 `json:"predictions"`
}
