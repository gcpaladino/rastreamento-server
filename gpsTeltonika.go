package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"time"
	ts "time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/db"
	"google.golang.org/api/option"

	"github.com/kelvins/geocoder"
)

const (
	CONN_PORT = "7098"
)

const PRECISION = 10000000.0

// Struct for Mongo GeoJSON
type Location struct {
	Type        string
	Coordinates []float64
}

// Record Schema
type Record struct {
	Imei      string
	latitude  float64
	longitude float64
	Time      time.Time
	Angle     int16
	Speed     int16
}

type coordatual struct {
	Imei                    string  `bson:"imei" json:"imei"`
	LatitudeDecimalDegrees  float64 `bson:"latitudeDecimalDegrees" json:"latitudeDecimalDegrees"`
	LatitudeHemisphere      string  `bson:"latitudeHemisphere" json:"latitudeHemisphere"`
	LongitudeDecimalDegrees float64 `bson:"longitudeDecimalDegrees" json:"longitudeDecimalDegrees"`
	LongitudeHemisphere     string  `bson:"longitudeHemisphere" json:"longitudeHemisphere"`
	SatelliteFixStatus      string  `bson:"satelliteFixStatus" json:"satelliteFixStatus"`
	Speed                   float64 `bson:"speed" json:"speed"`
	Time                    string  `bson:"time" json:"time"`
}

var (
	app           = initializeAppWithServiceAccount()
	dbFB          = initDatabase()
	coordAtualRef = dbFB.NewRef("coordatual")
)

func initializeAppWithServiceAccount() *firebase.App {
	opt := option.WithCredentialsFile("config/rastreamento-firebase-adminsdk.json")
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}
	return app
}

func initFirestore() *firestore.Client {
	client, err := app.Firestore(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	return client
}

func initDatabase() *db.Client {
	url := "https://rastreamento-ac921.firebaseio.com"
	client, err := app.DatabaseWithURL(context.Background(), url)
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	return client
}

func main() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", ":"+CONN_PORT)

	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	// Close the listener when the application closes.
	//defer l.Close()

	fmt.Println("Listening on " + CONN_PORT)
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	var b []byte
	var imei string

ILOOP:
	for {
		// Make a buffer to hold incoming data.
		buf := make([]byte, 2048)

		// Read the incoming connection into the buffer.
		size, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading:", err.Error())
			break
		}

		// Send a response if known IMEI and matches IMEI size
		//if knownIMEI {

		message := hex.EncodeToString(buf[2:size])
		fmt.Println("----------------------------------------")
		fmt.Println("Data From:", conn.RemoteAddr().String())
		fmt.Println("Size of message: ", size)
		fmt.Println("Message:", message)

		if size == 17 {
			imei = string(buf[2:size])
			fmt.Println("element imei string", imei)
			b = []byte{1} // 0x01 if we accept the message
			conn.Write(b)
		} else {
			elements, err := parseData(buf, size, imei)
			if err != nil {
				fmt.Println("Error while parsing data", err)
				conn.Close()
				break ILOOP
			}

			// if len(elements) > 0 {
			element := elements[0]
			fmt.Println("element save", element)

			SaveOrUpdateCoordinates(imei, element.latitude, "S", element.longitude, "W", "", float64(element.Speed), conn)

			// addresses, error := geoCodingLatLong(element.latitude, element.longitude)
			// if error != nil {
			// 	SaveOrUpdateCoordinates(imei, element.latitude, "S", element.longitude, "W", "", float64(element.Speed), conn)
			// } else {
			// 	SaveOrUpdateCoordinates(imei, element.latitude, "S", element.longitude, "W", addresses[0].FormattedAddress, float64(element.Speed), conn)
			// }
			// } else {
			b = []byte{0} // 0x00 if we decline the message
			conn.Write(b)
			// }

			break ILOOP
		}
	}
}

func parseData(data []byte, size int, imei string) (elements []Record, err error) {
	reader := bytes.NewBuffer(data)
	fmt.Println("Imei:", imei)

	// Header
	reader.Next(4)                                    // 4 Zero Bytes
	dataLength, err := streamToInt32(reader.Next(4))  // Header
	reader.Next(1)                                    // CodecID
	recordNumber, err := streamToInt8(reader.Next(1)) // Number of Records
	fmt.Println("Length of data:", dataLength)

	elements = make([]Record, recordNumber)

	var i int8 = 0
	for i < recordNumber {
		timestamp, err := streamToTime(reader.Next(8)) // Timestamp
		reader.Next(1)                                 // Priority

		// GPS Element
		longitudeInt, err := streamToInt32(reader.Next(4)) // Longitude
		longitude := float64(longitudeInt) / PRECISION
		latitudeInt, err := streamToInt32(reader.Next(4)) // Latitude
		latitude := float64(latitudeInt) / PRECISION

		reader.Next(2)                              // Altitude
		angle, err := streamToInt16(reader.Next(2)) // Angle
		reader.Next(1)                              // Satellites
		speed, err := streamToInt16(reader.Next(2)) // Speed

		if err != nil {
			fmt.Println("Error while reading GPS Element")
			break
		}

		elements[i] = Record{
			imei,
			latitude,
			longitude,
			timestamp,
			angle,
			speed}

		// IO Events Elements

		reader.Next(1) // ioEventID
		reader.Next(1) // total Elements

		stage := 1
		for stage <= 4 {
			stageElements, err := streamToInt8(reader.Next(1))
			if err != nil {
				break
			}

			var j int8 = 0
			for j < stageElements {
				reader.Next(1) // elementID

				switch stage {
				case 1: // One byte IO Elements
					_, err = streamToInt8(reader.Next(1))
				case 2: // Two byte IO Elements
					_, err = streamToInt16(reader.Next(2))
				case 3: // Four byte IO Elements
					_, err = streamToInt32(reader.Next(4))
				case 4: // Eigth byte IO Elements
					_, err = streamToInt64(reader.Next(8))
				}
				j++
			}
			stage++
		}

		if err != nil {
			fmt.Println("Error while reading IO Elements")
			break
		}

		fmt.Println("Timestamp:", timestamp, "Longitude:", longitude, "Latitude:", latitude)

		i++
	}

	// Once finished with the records we read the Record Number and the CRC

	_, err = streamToInt8(reader.Next(1))  // Number of Records
	_, err = streamToInt32(reader.Next(4)) // CRC

	return
}

func SaveOrUpdateCoordinates(
	imei string,
	latitudeDecimalDegrees float64,
	latitudeHemisphere string,
	longitudeDecimalDegrees float64,
	longitudeHemisphere string,
	address string,
	speed float64,
	conn net.Conn) {

	result, err := coordAtualRef.OrderByChild("imei").EqualTo(imei).GetOrdered(context.Background())
	if err != nil {
		log.Fatalf("error coordatual get: %v\n", err)
	}

	if len(result) > 0 {
		for _, r := range result {
			var c coordatual
			if err := r.Unmarshal(&c); err != nil {
				log.Fatalln("Error updating child:", err)
			}
			if err := dbFB.NewRef("coordatual/"+r.Key()).Update(context.Background(), map[string]interface{}{
				"imei":                    imei,
				"latitudeDecimalDegrees":  latitudeDecimalDegrees,
				"latitudeHemisphere":      latitudeHemisphere,
				"longitudeDecimalDegrees": longitudeDecimalDegrees,
				"longitudeHemisphere":     longitudeHemisphere,
				"satelliteFixStatus":      "A",
				"address":                 address,
				"speed":                   speed,
				"time":                    ts.Now(),
			}); err != nil {
				log.Fatalln("Error updating coordatual:", err)
			} else {
				log.Printf("Atualizou com sucesso em coordatual", imei)
			}
		}
	} else {
		_, err := coordAtualRef.Push(context.Background(), map[string]interface{}{
			"imei":                    imei,
			"latitudeDecimalDegrees":  latitudeDecimalDegrees,
			"latitudeHemisphere":      latitudeHemisphere,
			"longitudeDecimalDegrees": longitudeDecimalDegrees,
			"longitudeHemisphere":     longitudeHemisphere,
			"satelliteFixStatus":      "A",
			"address":                 address,
			"speed":                   speed,
			"time":                    ts.Now(),
		})
		if err != nil {
			log.Fatalln("Error save coordatual:", err)
		} else {
			log.Printf("Salvou com sucesso em coordatual", imei)
		}
	}

	fmt.Println("Imei salvo " + imei)

	SaveCoordinates(
		imei,
		latitudeDecimalDegrees,
		latitudeHemisphere,
		longitudeDecimalDegrees,
		longitudeHemisphere,
		address,
		speed,
		conn,
	)
}

func SaveCoordinates(
	imei string,
	latitudeDecimalDegrees float64,
	latitudeHemisphere string,
	longitudeDecimalDegrees float64,
	longitudeHemisphere string,
	address string,
	speed float64,
	conn net.Conn) {

	clientFB := initFirestore()
	coordinatesRef := clientFB.Collection("coordinates")

	defer clientFB.Close()
	defer conn.Close()

	coordinatesRef.Add(context.Background(), map[string]interface{}{
		"imei":                    imei,
		"latitudeDecimalDegrees":  latitudeDecimalDegrees,
		"latitudeHemisphere":      latitudeHemisphere,
		"longitudeDecimalDegrees": longitudeDecimalDegrees,
		"longitudeHemisphere":     longitudeHemisphere,
		"satelliteFixStatus":      "A",
		"address":                 address,
		"speed":                   speed,
		"timestamp":               firestore.ServerTimestamp,
	})

	log.Println("Salvou em coordinates:", imei)
}

func streamToUInt8(data []byte) (uint8, error) {
	var y uint8
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &y)
	return y, err
}

func streamToInt8(data []byte) (int8, error) {
	var y int8
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &y)
	return y, err
}

func streamToInt16(data []byte) (int16, error) {
	var y int16
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &y)
	return y, err
}

func streamToInt32(data []byte) (int32, error) {
	var y int32
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &y)
	if y>>31 == 1 {
		y *= -1
	}
	return y, err
}

func streamToInt64(data []byte) (int64, error) {
	var y int64
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &y)
	return y, err
}

func streamToFloat32(data []byte) (float32, error) {
	var y float32
	err := binary.Read(bytes.NewReader(data), binary.BigEndian, &y)
	return y, err
}

func twos_complement(input int32) int32 {
	mask := int32(math.Pow(2, 31))
	return -(input & mask) + (input &^ mask)
}

func streamToTime(data []byte) (time.Time, error) {
	miliseconds, err := streamToInt64(data)
	seconds := int64(float64(miliseconds) / 1000.0)
	nanoseconds := int64(miliseconds % 1000)

	return time.Unix(seconds, nanoseconds), err
}

func geoCodingLatLong(lat float64, long float64) ([]geocoder.Address, error) {
	// Set the latitude and longitude
	geocoder.ApiKey = "AIzaSyB0kNaF0Pwq3AD0r8LLKU5g0ABSzAxOKjg"
	location := geocoder.Location{
		Latitude:  lat,
		Longitude: long,
	}

	// Convert location (latitude, longitude) to a slice of addresses
	addresses, err := geocoder.GeocodingReverse(location)
	return addresses, err
}
