package main

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/stianeikeland/go-rpio"
	"golang.org/x/exp/io/i2c"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const screenWidth = 132

// var font map[rune][]byte

var dev *i2c.Device
var rng *rand.Rand

var btn1, btn2, btn3, joyUp, joyLeft, joyRight, joyDown, joyPress rpio.Pin

var ttc = transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)

func round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func bytefmt(size uint64) string {
	var suffixes [5]string
	suffixes[0] = "B"
	suffixes[1] = "KB"
	suffixes[2] = "MB"
	suffixes[3] = "GB"
	suffixes[4] = "TB"

	base := math.Log(float64(size)) / math.Log(1024)
	getSize := round(math.Pow(1024, base-math.Floor(base)), .5, 2)
	getSuffix := suffixes[int(math.Floor(base))]
	return strconv.FormatFloat(getSize, 'f', -1, 64) + " " + string(getSuffix)
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
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

func initDevice() {
	err := rpio.Open()
	if err != nil {
		panic(err)
	}

	initInput()

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

	pinRST.High()
	time.Sleep(100 * time.Millisecond)

	pinRST.Low()
	time.Sleep(100 * time.Millisecond)

	pinRST.High()
	time.Sleep(100 * time.Millisecond)

	writeRegister(0xae)       // turn off oled panel
	writeRegister(0x02, 0x10) // set low column address
	writeRegister(0x40, 0x00) // set start line address
	writeRegister(0xad, 0x8b) // set charge pump
	writeRegister(0x81, 0xff) // set contrast control register
	writeRegister(0xa0, 0xc8) // set display mirror and direction (0xa0 / 0xa1) (0xc0 / 0xc8)
	// writeRegister(0xa1, 0xc0)
	writeRegister(0xa6)       // set normal/reverse (0xa6 / 0xa7)
	writeRegister(0xa8, 0x3f) // set multiplex ratio (1 to 64)
	writeRegister(0xd3, 0x00) // set display offset
	writeRegister(0xd5, 0x80) // set display clock divide ratio/oscillator frequency
	writeRegister(0xd9, 0xff) // set pre-charge period
	writeRegister(0xda, 0x12) // set com pins hardware configuration
	writeRegister(0xdb, 0x40) // set vcomh
	writeRegister(0xaf)       // turn on oled panel
}

func initInput() {
	btn1 = rpio.Pin(21)
	btn1.Input()
	btn1.PullUp()

	btn2 = rpio.Pin(20)
	btn2.Input()
	btn2.PullUp()

	btn3 = rpio.Pin(16)
	btn3.Input()
	btn3.PullUp()

	joyUp = rpio.Pin(6)
	joyUp.Input()
	joyUp.PullUp()

	joyDown = rpio.Pin(19)
	joyDown.Input()
	joyDown.PullUp()

	joyLeft = rpio.Pin(5)
	joyLeft.Input()
	joyLeft.PullUp()

	joyRight = rpio.Pin(26)
	joyRight.Input()
	joyRight.PullUp()

	joyPress = rpio.Pin(13)
	joyPress.Input()
	joyPress.PullUp()
}

func clear() {
	for p := 0; p < 8; p++ {
		writeRegister(0xB0+byte(p), 0x02, 0x10)
		t := make([]byte, 129)
		t[0] = 0x40
		dev.Write(t)
	}
}

func wait() {
	for {
		if joyPress.Read() == rpio.Low {
			break
		}
	}

	time.Sleep(250 * time.Millisecond)
}

func padLeft(d []byte) []byte {
	padding := make([]byte, (128 - len(d)))
	return append(padding, d...)
}

func padRight(d []byte) []byte {
	padding := make([]byte, (128 - len(d)))
	return append(d, padding...)
}
func padCenter(d []byte) []byte {
	padding := make([]byte, (128-len(d))/2)
	return append(padding, d...)
}

func buildText(text string) [][]byte {
	var pages [][]byte

	for _, t := range strings.Split(text, "\n") {
		var enc []byte

		for _, w := range strings.Split(t, " ") {
			encoded := encodeText(w + " ")
			// the extra `-4` at the end is to get rid of the trailing <space>
			if len(enc)+len(encoded) > screenWidth {
				pages = append(pages, enc)
				enc = encoded
			} else {
				enc = append(enc, encoded...)
			}
		}

		if len(enc) > 0 {
			pages = append(pages, enc[:len(enc)-4])
			enc = []byte{}
		}
	}

	// trim pages
	if len(pages[len(pages)-1]) == 0 {
		pages = pages[:len(pages)-1]
	}

	return pages
}

func encodeText(s string) []byte {
	res := []byte{}

	s = strings.Replace(s, "\t", "  ", -1)

	s, _, err := transform.String(ttc, s)
	if err != nil {
		panic(err)
	}

	for _, r := range []rune(s) {
		res = append(res, font[r]...)
	}

	return res
}

func printTextWithTitle(title, text string, offset ...int) int {
	printText(title, 0)
	printText(strings.Join(make([]string, screenWidth/4), "-"), 1)

	return printText(text, 2) - 2
}

func printText(text string, _offset ...int) int {
	var offset int

	if len(_offset) == 0 {
		offset = 0
	} else {
		offset = _offset[0]
	}

	pages := buildText(text)

	start := 0
	end := len(pages)
	if end > 7 {
		end = 7
	}

	for _, page := range pages[start:end] {
		writeRegister(0xB7-byte(offset), 0x02, 0x10)
		writeData(page)
		offset++
	}

	return offset
}

func printDots() {
	var m = 1
	for {
		if btn1.Read() == rpio.Low {
			m = 0
		} else if btn2.Read() == rpio.Low {
			m = 1
		} else if btn3.Read() == rpio.Low {
			m = 2
		} else if joyPress.Read() == rpio.Low {
			break
		}

		for p := 0; p < 8; p++ {
			writeRegister(0xB0+byte(p), 0x02, 0x10)

			t := make([]byte, 129)
			rand.Read(t)

			for i := range t {
				switch m {
				case 1:
					if i%2 != 0 {
						t[i] = 0
					} else {
						t[i] &= 0xaa
					}
				case 2:
					if i%4 != 0 {
						t[i] = 0
					} else {
						t[i] &= 0x88
					}
				case 3:
					t[i] = 0
				default:
				}
			}

			t[0] = 0x40
			dev.Write(t)
		}
	}
}

func printFont() {
	keys := make([]int, 0)
	for k := range font {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	fontString := ""
	for i, k := range keys {
		fontString += string(k)
		if i%(screenWidth/4) == (screenWidth/4)-1 {
			fontString += "\n"
		}
	}

	printText(fontString, 0)
}

func printRandomRune() {
	var enc []byte

	keys := make([]int, 0)
	for k := range font {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	// for range `60 fps` { ... }
	for range time.NewTicker(time.Second / 60).C {
		if joyPress.Read() == rpio.Low {
			break
		}

		for p := 7; p >= 0; p-- {
			for len(enc) <= screenWidth {
				k := keys[rand.Intn(len(keys))]
				enc = append(enc, font[rune(k)]...)
			}

			writeRegister(0xB0+byte(p), 0x02, 0x10)
			writeData(enc)
			enc = []byte{}
		}
	}
}

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	initDevice()
	clear()
}

func main() {
	defer rpio.Close()
	defer dev.Close()

	tweets := [][]string{
		{
			"Robin Ward (eviltrout)",
			"Bad Blood is a stressful read for me. Theranos has all the bad elements of every startup I've ever encountered, but amplified 100x.",
		},
		{
			"Ashley McNamara (ashleymcnamara)",
			"I just wasted 1/4 of a bottle of all natural cleaner to kill a spider, so, from now on I'm only buying harsh cleaning chemicals because bleach wouldn't have let me down like this...",
		},
		{
			"Tommy Refenes (TommyRefenes)",
			"I'm an Elon Musk fan but it is kind of funny thinking that he built that submarine and took it all the way there and they're like oh thanks and then I just put it off to the side",
		},
		{
			"Mark Nottingham (mnot)",
			"TIL: Chrome disables the browser cache if it thinks it's on a broken HTTPS connection (e.g., invalid cert)",
		},
	}

	// for range time.NewTicker(time.Millisecond * 100).C {
	// 	v, _ := mem.VirtualMemory()

	// 	printTextWithTitle("memory",
	// 		strings.Join([]string{
	// 			fmt.Sprintf("Total: %v", bytefmt(v.Total)),
	// 			fmt.Sprintf("Free: %v", bytefmt(v.Free)),
	// 			fmt.Sprintf("Used: %.2f%%", v.UsedPercent),
	// 		}, "\n"))
	// }
	// return

	for {
		// todo: handle non ascii char
		// printText("¯\\_(ツ)_/¯", 0)
		// printText("45.5017° N, 73.5673° W", 1)
		// wait()
		// clear()

		// printFont()
		// wait()
		// clear()

		// printRandomRune()
		// time.Sleep(100 * time.Millisecond)
		// clear()

		// printDots()
		// time.Sleep(100 * time.Millisecond)
		// clear()

		i := 0
		offset := 0
		keep := true

		for keep {
			tweet := tweets[i]
			printTextWithTitle(tweet[0], tweet[1], offset)
			for {
				if joyLeft.Read() == rpio.Low && i > 0 {
					i--
					break
				} else if joyRight.Read() == rpio.Low && i < len(tweets)-1 {
					i++
					break
				}
			}

			time.Sleep(250 * time.Millisecond)

			clear()
		}

	}
}
