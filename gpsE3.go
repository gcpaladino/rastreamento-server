package main

import (
	"context"
	"fmt"
	"log"
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
	listen, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		log.Fatalf("Socket listen port %d failed,%s", port, err)
		os.Exit(1)
	}

	defer listen.Close()

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Fatalln(err)
			continue
		}
		go handleConnection(conn)
	}

}

//*ET,SN,HB,A/V,YYMMDD,HHMMSS,Latitude,Longitude,Speed,Course,Status,Signal,Power,Fuel,Mileage,Al titude,GPSdata,[RFID],[Temperature],voltage,Sat #
//*ET,354522180399565,HB,A,14091D,14261F,80D1D404,81AF2A84,1E78,7D00,40800000,20,100,00,0000002E,663,1229250520,0000000000,0000,13.80,11#

func handleConnection(conn net.Conn) {

ILOOP:
	for {
		// Make a buffer to hold incoming data.
		buf := make([]byte, 1024)

		// Read the incoming connection into the buffer.
		size, err := conn.Read(buf)
		data := string(buf[:size])

		if err == nil {
			items := strings.Split(data, ",")
			if len(items) > 0 {
				model := items[0]
				if model == "*ET" {
					if len(items) > 0 {
						imei := items[1]
						cmd := items[2]
						if cmd == "TX" {
							//isDataValid. Length is 1, A means GPS data is available, V mean data is unavailable, L means base station data
							//isDataValid := items[3]
							conn.Write(buf[:size])
						} else if cmd == "MQ" {
							conn.Write(buf[:size])
						} else if cmd == "HB" {
							hexLat := items[6]
							hexLog := items[7]

							x := tratarCoordEx3(hexLat)
							y := tratarCoordEx3(hexLog)
							//conn.Close()
							//veloGPRS, _ := strconv.ParseFloat(items[19], 32)
							//velocidadekm := veloGPRS * 1.852

							SaveOrUpdateCoordinates(imei, x, "", y, "", "", float64(0))

							// addresses, err := geoCodingLatLong(x, y)
							// if err != nil {
							// 	SaveOrUpdateCoordinates(imei, x, "", y, "", "", float64(0), conn)
							// } else {
							// 	SaveOrUpdateCoordinates(imei, x, "", y, "", addresses[0].FormattedAddress, float64(0), conn)
							// }
							conn.Write(buf[:size])
							//break ILOOP
						}
					}
				}
			} //else {
			//conn.Close()
			//break ILOOP
			//}
		} else {
			conn.Close()
			break ILOOP
		}
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
		speed,
		address,
	)
}

func SaveCoordinates(
	imei string,
	latitudeDecimalDegrees float64,
	latitudeHemisphere string,
	longitudeDecimalDegrees float64,
	longitudeHemisphere string,
	speed float64,
	address string) {

	clientFB := initFirestore()
	coordinatesRef := clientFB.Collection("coordinates")

	defer clientFB.Close()

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

func tratarCoordEx3(hexaString string) float64 {
	x, _ := strconv.ParseInt(cleanHex(hexaString), 16, 64)
	xFloat, _ := strconv.ParseFloat(fmt.Sprintf("%d", x), 64)
	if hexaString[0:1] == "8" {
		return (xFloat / 600000) * -1
	} else {
		return (xFloat / 600000)
	}
}

func cleanHex(hexaString string) string {
	if hexaString[0:1] == "8" {
		if hexaString[1:2] == "0" {
			return hexaString[2:]
		} else {
			return hexaString[1:]
		}
	} else {
		return hexaString
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
