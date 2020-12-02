package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	ts "time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/db"
	"google.golang.org/api/option"

	"github.com/kelvins/geocoder"
)

const (
	CONN_HOST = "0.0.0.0"
	CONN_PORT = "7098"
	CONN_TYPE = "tcp4"
	PRECISION = 10000000.0
)

var (
	app           = initializeAppWithServiceAccount()
	dbFB          = initDatabase()
	coordAtualRef = dbFB.NewRef("coordatual")
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
	l, err := net.Listen(CONN_TYPE, CONN_HOST+":"+CONN_PORT)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}

	// Close the listener when the application closes.
	defer l.Close()

	fmt.Println("Listening on " + CONN_HOST + ":" + CONN_PORT)
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

ILOOP:
	for {
		// Make a buffer to hold incoming data.
		buf := make([]byte, 2048)

		// Read the incoming connection into the buffer.
		size, err := conn.Read(buf)
		//data := string(buf[:size])

		if err == nil {
			message := hex.EncodeToString(buf[:size])
			log.Println("Receive string and byte size:", message, size)
			if size == 18 {
				//7878 0d 01 0 3595100800050 0100e2116b0d0a
				conn.Write(buf[:size])
			} else if size == 65 {
				//7878-3c-22-01-0365119068397451-1307190c0d07-1307190c0d08-cb-028470ba-05078c1c-00-3837-09-02d4-0a-24c3-0001bb-46-0514-2a-60-0002-000000-000000-002f-22b7-0d0a
				//7878-3c-22-01-0359510080006099-140a18103a2a-140a18103a2a-cb-02723e74-05138433-00-3872-09-02d4-0a-0527-0013bd-46-04d1-28-54-0002-000a0c-000000-0037-8f36-0d0a

				reader := bytes.NewBuffer(buf[:size])

				// Header
				reader.Next(2)                                                // 2 Bytes - Start Bit
				packetLength, _ := streamToInt8(reader.Next(1))               // 1 Byte - Packet Length
				protocolNumber := message[6:8]                                // 1 Bytes - Protocol Number
				locationType := message[8:10]                                 // 1 Bytes - Location Source Type
				imei := message[11:26]                                        // 8 Bytes - Imei
				internalYear, _ := strconv.ParseInt(message[26:28], 16, 64)   // 1 Byte - Year Internal
				internalMonth, _ := strconv.ParseInt(message[28:30], 16, 64)  // 1 Byte - Month Internal
				internalDay, _ := strconv.ParseInt(message[30:32], 16, 64)    // 1 Byte - Day Internal
				internalHour, _ := strconv.ParseInt(message[32:34], 16, 64)   // 1 Byte - Hour Internal
				internalMinute, _ := strconv.ParseInt(message[34:36], 16, 64) // 1 Byte - Minute Internal
				internalSecond, _ := strconv.ParseInt(message[36:38], 16, 64) // 1 Byte - Second Internal

				// GPS Element
				gpsYear, _ := strconv.ParseInt(message[38:40], 16, 64)   // 1 Byte - Year GPS
				gpsMonth, _ := strconv.ParseInt(message[40:42], 16, 64)  // 1 Byte - Month GPS
				gpsDay, _ := strconv.ParseInt(message[42:44], 16, 64)    // 1 Byte - Day GPS
				gpsHour, _ := strconv.ParseInt(message[44:46], 16, 64)   // 1 Byte - Hour GPS
				gpsMinute, _ := strconv.ParseInt(message[46:48], 16, 64) // 1 Byte - Minute GPS
				gpsSecond, _ := strconv.ParseInt(message[48:50], 16, 64) // 1 Byte - Second GPS

				latitudeInt, _ := strconv.ParseInt(message[52:60], 16, 64) // Latitude
				latitude := float64(latitudeInt) / 30000 / 60 * -1

				longitudeInt, _ := strconv.ParseInt(message[60:68], 16, 64) // Longitude
				longitude := float64(longitudeInt) / 30000 / 60 * -1

				speed, _ := strconv.ParseInt(message[68:70], 16, 64) // Speed

				log.Println("Receive packetLength:", packetLength)
				log.Println("Receive protocolNumber:", protocolNumber)
				log.Println("Receive locationType:", locationType)
				log.Println("Receive imei:", imei)
				log.Println("Receive gpsDate:", internalDay, internalMonth, internalYear, internalHour, internalMinute, internalSecond)
				log.Println("Receive gpsDate:", gpsDay, gpsMonth, gpsYear, gpsHour, gpsMinute, gpsSecond)
				log.Println("Receive latitudeInt:", latitudeInt)
				log.Println("Receive longitudeInt:", longitudeInt)
				log.Println("Receive latitude:", latitude)
				log.Println("Receive longitude:", longitude)
				log.Println("Receive speed:", speed)

				SaveOrUpdateCoordinates(imei, latitude, "S", longitude, "W", "", float64(speed), conn)

				// addresses, err := geoCodingLatLong(latitude, longitude)
				// if err != nil {

				// 	log.Println("Error address: ", err)
				// } else {
				// 	SaveOrUpdateCoordinates(imei, latitude, "S", longitude, "W", addresses[0].FormattedAddress, float64(speed), conn)
				// 	log.Println("Address: ", addresses[0].FormattedAddress)
				// }

				break ILOOP
			}
		} else {
			log.Println("Receive error:", err)
			conn.Close()
			break ILOOP
		}
	}
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

	log.Println("Salvou em coordinates")

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
