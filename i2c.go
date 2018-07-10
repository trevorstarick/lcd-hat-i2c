package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/stianeikeland/go-rpio"
	"golang.org/x/exp/io/i2c"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var dev *i2c.Device
var rng *rand.Rand
var m uint = 1

var ttc = transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

func encodeText(s string) []byte {
	res := []byte{}

	s, _, err := transform.String(ttc, s)
	if err != nil {
		panic(err)
	}

	for _, r := range []rune(s) {
		res = append(res, font[r]...)
	}

	return res
}

func writeRegister(reg byte, data ...byte) {
	if err := dev.WriteReg(0x00, append([]byte{reg}, data...)); err != nil {
		panic(err)
	}
}

func writeData(data []byte) {
	if err := dev.Write(append([]byte{0x40}, data...)); err != nil {
		panic(err)
	}
}

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func main() {
	err := rpio.Open()
	if err != nil {
		panic(err)
	}
	defer rpio.Close()

	btn1 := rpio.Pin(21)
	btn1.Input()
	btn1.PullUp()

	btn2 := rpio.Pin(20)
	btn2.Input()
	btn2.PullUp()

	btn3 := rpio.Pin(16)
	btn3.Input()
	btn3.PullUp()

	joyUp := rpio.Pin(6)
	joyUp.Input()
	joyUp.PullUp()

	joyDown := rpio.Pin(19)
	joyDown.Input()
	joyDown.PullUp()

	joyLeft := rpio.Pin(5)
	joyLeft.Input()
	joyLeft.PullUp()

	joyRight := rpio.Pin(26)
	joyRight.Input()
	joyRight.PullUp()

	joyPress := rpio.Pin(13)
	joyPress.Input()
	joyPress.PullUp()

	pinRST := rpio.Pin(25)
	pinRST.Output()
	pinDC := rpio.Pin(24)
	pinDC.Output()
	pinCS := rpio.Pin(8)
	pinCS.Output()

	pinCS.Low()
	pinDC.Low()

	dev, err = i2c.Open(&i2c.Devfs{Dev: "/dev/i2c-1"}, 0x3c)
	if err != nil {
		panic(err)
	}
	defer dev.Close()

	pinRST.High()
	time.Sleep(100 * time.Millisecond)

	pinRST.Low()
	time.Sleep(100 * time.Millisecond)

	pinRST.High()
	time.Sleep(100 * time.Millisecond)

	writeRegister(0xAE)       //--turn off oled panel
	writeRegister(0x02, 0x10) //---set low column address
	writeRegister(0x40)       //--set start line address  Set Mapping RAM Display Start Line (0x00~0x3F)
	// writeRegister(0xad, 0x8b) //--set charge pump
	writeRegister(0x81, 0xff) //--set contrast control register
	writeRegister(0xA6, 0xc8) // set rotation and direction
	writeRegister(0xA6)       // set normal/reverse (0xa6-0xa7)
	writeRegister(0xA8, 0x3f) //--set multiplex ratio(1 to 64)
	writeRegister(0xD3, 0x00) //-set display offset    Shift Mapping RAM Counter (0x00~0x3F)
	writeRegister(0xd5, 0x80) //--set display clock divide ratio/oscillator frequency
	writeRegister(0xD9, 0xff) //--set pre-charge period
	writeRegister(0xDA, 0x12) //--set com pins hardware configuration
	writeRegister(0xDB, 0x40) //--set vcomh
	writeRegister(0xa4, 0xa6) // Disable Entire Display On (0xa4/0xa5)
	writeRegister(0xaf)

	// return

	const pageSize = int(64 / 8)
	// var screen [1024]byte
	var page [128]byte
	_ = page

	// clear screen
	for p := 0; p < pageSize; p++ {
		writeRegister(0xB0+byte(p), 0x02, 0x10)
		t := make([]byte, 129)
		t[0] = 0x40
		dev.Write(t)
	}

	var enc []byte

	// todo: handle non ascii char
	// writeRegister(0xB7, 0x02, 0x10)
	// enc = encodeText("¯\\_(ツ)_/¯")
	// enc = append(make([]byte, (128-len(enc))/2), enc...)
	// writeData(enc)
	// return

	// writeRegister(0xB7, 0x02, 0x10)
	// enc = encodeText("45.5017° N, 73.5673° W")
	// enc = append(make([]byte, (128-len(enc))/2), enc...)
	// writeData(enc)
	// return

	keys := make([]int, 0)
	for k := range font {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	// for range `60 fps` { ... }
	// for range time.NewTicker(time.Second / 60).C {
	// 	for p := 7; p >= 0; p-- {
	// 		for len(enc) <= 128 {
	// 			k := keys[rand.Intn(len(keys))]
	// 			enc = append(enc, font[rune(k)]...)
	// 		}

	// 		writeRegister(0xB0+byte(p), 0x02, 0x10)
	// 		writeData(enc)
	// 		enc = []byte{}
	// 	}
	// }

	// return

	type tweet struct {
		name   string
		handle string
		text   string
		date   time.Time
	}

	date, _ := time.Parse("3:04 PM - 2 Jan 2006", "10:03 PM - 8 Jul 2018")
	t := tweet{
		name:   "Mark Nottingham",
		handle: "mnot",
		text:   "TIL: Chrome disables the browser cache if it thinks it's on a broken HTTPS connection (e.g., invalid cert)",
		date:   date,
	}

	writeRegister(0xB7, 0x02, 0x10)
	enc = encodeText(fmt.Sprintf("%v (%v)", t.name, t.handle))
	enc = append(make([]byte, (128-len(enc))/2), enc...)
	writeData(enc)

	writeRegister(0xB6, 0x02, 0x10)
	enc = encodeText("-------------------------------")
	enc = append(make([]byte, (128-len(enc))/2), enc...)
	writeData(enc)

	p := 6
	for _, w := range strings.Split(t.text, " ") {
		encoded := encodeText(w + " ")
		if len(enc)+len(encoded) > 128-1 {
			writeRegister(0xB0+byte(p), 0x02, 0x10)
			writeData(enc)
			enc = []byte{}
			p--
		}

		enc = append(enc, encoded...)
	}

	writeRegister(0xB1, 0x02, 0x10)
	enc = encodeText("-------------------------------")
	enc = append(make([]byte, (128-len(enc))/2), enc...)
	writeData(enc)

	writeRegister(0xB0, 0x02, 0x10)
	enc = encodeText(t.date.Format("3:04 PM - 2 Jan 2006"))
	enc = append(make([]byte, (128-len(enc))/2), enc...)
	writeData(enc)
}
