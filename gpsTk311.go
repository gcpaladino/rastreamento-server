package main

import (
	"context"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	ts "time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/db"
	"google.golang.org/api/option"

	"github.com/kelvins/geocoder"
)

var msgLoad = `LOAD`
var msgON = `ON`

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

func SocketServer(port int) {
	listen, err := net.Listen("tcp4", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("Socket listen port %d failed,%s", port, err)
		os.Exit(1)
	}

	defer listen.Close()

	//log.Printf("Begin listen port: %d", port)

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatalln(err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
ILOOP:
	for {
		buf := make([]byte, 1024)

		// Read the incoming connection into the buffer.
		size, err := conn.Read(buf)
		data := string(buf[:size])
		if err == nil {
			log.Println("Receive handle:", data)
			parseData(data, conn)
			//break ILOOP
		} else {
			break ILOOP
		}
	}
}

//imei:865011031499958,tracker,201015164347,,F,194347.00,A,2337.12530,S,04639.79121,W,0.047,0,,1,,,,;
func parseData(data string, conn net.Conn) {

	if strings.Contains(data, "##") {
		conn.Write([]byte(msgLoad + "\n"))
	} else if strings.Contains(data, "imei") {
		items := strings.Split(data, ",")
		if len(items) > 0 {
			cmd := items[1]
			if cmd == "tracker" {
				imeis := strings.Split(items[0], ":")
				imei := imeis[1]
				latitudeDecimalDegrees := items[7]
				latitudeHemisphere := items[8]
				longitudeDecimalDegrees := items[9]
				longitudeHemisphere := items[10]

				latitude := ParseGPS("0" + latitudeDecimalDegrees + " " + latitudeHemisphere)
				longitude := ParseGPS(longitudeDecimalDegrees + " " + longitudeHemisphere)

				SaveOrUpdateCoordinates(imei, latitude, latitudeHemisphere, longitude, longitudeHemisphere, "", 0.0)

				// addresses, err := geoCodingLatLong(latitude, longitude)
				// if err != nil {
				// 	SaveOrUpdateCoordinates(imei, latitude, latitudeHemisphere, longitude, longitudeHemisphere, "", 0.0, conn)
				// } else {
				// 	SaveOrUpdateCoordinates(imei, latitude, latitudeHemisphere, longitude, longitudeHemisphere, addresses[0].FormattedAddress, 0.0, conn)
				// }
			}
		}
	} else {
		conn.Write([]byte(msgON + "\n"))
	}
}

func SaveOrUpdateCoordinates(
	imei string,
	latitudeDecimalDegrees float64,
	latitudeHemisphere string,
	longitudeDecimalDegrees float64,
	longitudeHemisphere string,
	address string,
	speed float64) {

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
		}
	}

	SaveCoordinates(
		imei,
		latitudeDecimalDegrees,
		latitudeHemisphere,
		longitudeDecimalDegrees,
		longitudeHemisphere,
		address,
		speed,
	)
}

func SaveCoordinates(
	imei string,
	latitudeDecimalDegrees float64,
	latitudeHemisphere string,
	longitudeDecimalDegrees float64,
	longitudeHemisphere string,
	address string,
	speed float64) {

	clientFB := initFirestore()
	coordinatesRef := clientFB.Collection("coordinates")

	defer clientFB.Close()
	//defer conn.Close()

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
}

// ParseGPS parses a GPS/NMEA coordinate.
// e.g 15113.4322S
func ParseGPS(s string) float64 {
	parts := strings.Split(s, " ")
	if len(parts) != 2 {
		return 0
	}
	dir := parts[1]
	value, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}

	degrees := math.Floor(value / 100)
	minutes := value - (degrees * 100)
	value = degrees + minutes/60

	if dir == "N" || dir == "E" {
		return value
	} else if dir == "S" || dir == "W" {
		return value * -1
	} else {
		return 0
	}
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

func main() {
	port := 7098
	SocketServer(port)
}
