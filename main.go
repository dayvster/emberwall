// emberwall makes random procedural wallpapers (black & orange Ember by default, any palette on request).
//
//	emberwall                        one random wallpaper at your screen size
//	emberwall -n 12                  a dozen of them
//	emberwall -style topo,flow -n 6  only those styles
//	emberwall -style topo -seed 42   regenerate one you liked (name shows style + seed)
//	emberwall -palette ice,rose      other colours (or -palette random)
//	emberwall -color '#3fa9f5'       your own colour (1-3 hex colours, comma-separated)
//	emberwall -logo arch             a distro logo on top of any style
//	emberwall -svg ./mylogo.svg      your own SVG on top, with the same -logo-pos/-size/-color flags
//	emberwall -set                   also set the (last) result as your wallpaper
//	emberwall -list                  list the styles, palettes and logos
//
// Standard library only, so `go build` works the same on macOS and Linux.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Style struct {
	Name, Desc string
	Fn         func(c *Canvas, r *rand.Rand) string
}

var styles = []Style{
	{"topo", "topographic contour lines, brighter every 5th line", topo},
	{"synthwave", "striped retro sun, ridged mountains, neon floor grid", synthwave},
	{"dunes", "layered dunes with orange rim light and a low sun", dunes},
	{"eclipse", "black disc with a burning, streaky corona", eclipse},
	{"flow", "thousands of thin lines tracing a flow field", flow},
	{"ridges", "stacked ridgeline plot rising in the middle", ridges},
	{"halftone", "halftone dot field swelling across the screen", halftone},
	{"embers", "sparks and bokeh drifting up from a glow", embers},
	{"matrix", "digital rain: falling columns of glowing code, three depth layers", matrix},
	{"lava", "cracked dark rock with glowing seams between the plates", lava},
	{"circuit", "circuit-board traces with solder pads and signal pulses", circuit},
	{"truchet", "endless maze of glowing quarter-circle tiles", truchet},
	{"lowpoly", "faceted triangle terrain lit from one side", lowpoly},
	{"nebula", "deep-space gas cloud with dust lanes and spiked stars", nebula},
	{"skyline", "night city silhouette with lit windows and a reflection", skyline},
	{"planet", "ringed planet half in shadow with a glowing limb", planet},
	{"wireframe", "3D grid mountains fading into fog", wireframe},
	{"orbits", "spirograph of hundreds of thin overlapping ellipses", orbits},
	{"crt", "oscilloscope traces on a curved, scanlined old monitor", crtStyle},
	{"digital", "motherboard: chips, parallel bus lines and streaming data packets", digital},
	{"fractal", "Julia sets and deep Mandelbrot zooms with glowing filaments", fractal},
	{"mandelbulb", "3D Mandelbrot: an organic, coral-like bulb (raymarched)", mandelbulb},
	{"bulbwide", "3D mandelbulb up close, filling the full screen width", bulbwide},
	{"menger", "3D Menger sponge: a cube of endlessly recursing square holes", menger},
	{"mandelbox", "3D Mandelbox: intricate, mechanical alien architecture", mandelbox},
}

func main() {
	n := flag.Int("n", 1, "how many wallpapers to make")
	style := flag.String("style", "random", "style name, comma-separated list, or random (see -list)")
	seed := flag.Int64("seed", 0, "seed (0 = random); same style + seed + size = same image")
	size := flag.String("size", "auto", "WIDTHxHEIGHT, or auto = your screen")
	out := flag.String("o", defaultOut(), "output folder")
	grain := flag.Float64("grain", 1, "film grain amount (0 = none)")
	crt := flag.Bool("crt", false, "old curved-monitor look on top of any style (scanlines, curvature, glitches)")
	set := flag.Bool("set", false, "set the last result as wallpaper")
	open := flag.Bool("open", false, "open the results in your image viewer")
	palette := flag.String("palette", "ember", "palette name, comma-separated list, or random (see -list)")
	color := flag.String("color", "", "custom colours instead of a palette: 1-3 hex, e.g. '#3fa9f5' or '#ff4f7b,#ffd23f'")
	colors := flag.String("colors", "", "several accent colours blended across the image (2-8 hex), e.g. Dracula: '#bd93f9,#ff79c6,#8be9fd,#50fa7b'")
	list := flag.Bool("list", false, "list styles, palettes and logos, then exit")
	logoName := flag.String("logo", "", "put a distro logo on top: name, comma-separated list, or random (see -list)")
	svgPath := flag.String("svg", "", "use a custom SVG file as the logo; works with -logo-pos, -logo-size and -logo-color")
	logoPos := flag.String("logo-pos", "center", "where the logo goes: center, left, right or corner")
	logoSize := flag.String("logo-size", "medium", "logo size: tiny, small, medium, large (or big) or huge")
	logoColor := flag.String("logo-color", "", "logo in exactly this colour, e.g. '#1793d1', or original = the distro's own colour (default: follows the palette)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "emberwall: random procedural wallpapers\n\nusage: emberwall [flags]")
		flag.PrintDefaults()
		printLists(os.Stderr)
		fmt.Fprintln(os.Stderr, "\nexamples:\n  emberwall -n 5 -open\n  emberwall -style topo -palette ice\n  emberwall -color '#ff4f7b,#ffd23f' -n 3\n  emberwall -colors '#bd93f9,#ff79c6,#8be9fd,#50fa7b'   (several colours, e.g. Dracula)\n  emberwall -style dunes -seed 11        (re-make one: style + seed are in the file name)\n  emberwall -style synthwave -logo arch  (a distro logo on top; -logo-pos center|left|right|corner)\n  emberwall -logo arch -logo-size small   (tiny, small, medium, large or huge)\n  emberwall -logo arch -logo-color '#1793d1'  (logo in an exact colour, whatever the palette)\n  emberwall -logo random -logo-color original  (each distro in its own brand colour)\n  emberwall -svg ~/my-logo.svg -logo-pos corner -logo-size large  (your own SVG on top)")
	}
	flag.Parse()

	if *list {
		printLists(os.Stdout)
		return
	}

	w, h, err := parseSize(*size)
	if err != nil {
		fail(err)
	}

	pool, err := parseStyles(*style)
	if err != nil {
		fail(err)
	}
	pals, err := parsePalettes(*palette, *color, *colors)
	if err != nil {
		fail(err)
	}
	logo, err := parseLogoChoice(*logoName, *logoPos, *logoSize, *logoColor)
	if err != nil {
		fail(err)
	}
	if *svgPath != "" {
		l, err := loadCustomLogo(*svgPath)
		if err != nil {
			fail(err)
		}
		logo.Pool = append(logo.Pool, l)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}

	masterSeed := *seed
	if masterSeed == 0 {
		masterSeed = time.Now().UnixNano()
	}
	master := rand.New(rand.NewSource(masterSeed))

	var paths []string
	for i := 0; i < *n; i++ {
		st := pool[(master.Intn(len(pool))+i)%len(pool)]
		s := master.Int63n(1e9) + 1
		if *seed != 0 && i == 0 {
			s = *seed
		}
		pal := pals[master.Intn(len(pals))]
		t0 := time.Now()
		c := NewCanvas(w, h, Black)
		c.Accents = max(len(pal.Multi), 1)
		c.SeedTags(s)
		desc := st.Fn(c, rand.New(rand.NewSource(s)))
		if *crt && st.Name != "crt" {
			c.CRT(rand.New(rand.NewSource(s+7)), 0.8)
		}
		logoTag := ""
		l, logoCol, hasLogo := logo.pick(master)
		if hasLogo {
			logoTag = "-" + l.Name
		}
		if hasLogo && logoCol == nil { // palette-coloured: drawn in Ember, re-mapped with the rest
			if err := c.DrawLogo(l, logo.Pos, logo.Size, nil); err != nil {
				fail(err)
			}
		}
		pal.Apply(c, uint64(s))
		if hasLogo && logoCol != nil { // exact colour: drawn after the palette so it stays as given
			if err := c.DrawLogo(l, logo.Pos, logo.Size, logoCol); err != nil {
				fail(err)
			}
		}
		img := c.Finish(*grain, uint64(s))
		tag := ""
		if pal.Name != "ember" {
			tag = "-" + pal.Name
		}
		path := filepath.Join(*out, fmt.Sprintf("ember-%s%s%s-%d.png", st.Name, tag, logoTag, s))
		if err := savePNG(path, img); err != nil {
			fail(err)
		}
		paths = append(paths, path)
		fmt.Printf("%s  (%dx%d, %.1fs)  %s\n", path, w, h, time.Since(t0).Seconds(), desc)
	}
	if len(paths) > 1 || *seed == 0 {
		fmt.Printf("\nlike one? re-make it with:  emberwall -style <style> -palette <palette> -seed <number>  (all in the file name)\n")
	}
	if *set {
		if err := setWallpaper(paths[len(paths)-1]); err != nil {
			fmt.Fprintln(os.Stderr, "could not set wallpaper:", err)
		}
	}
	if *open {
		opener := "xdg-open"
		if runtime.GOOS == "darwin" {
			opener = "open"
		}
		for _, p := range paths {
			_ = exec.Command(opener, p).Start()
		}
	}
}

func printLists(w *os.File) {
	fmt.Fprintln(w, "\nstyles:")
	for _, s := range styles {
		fmt.Fprintf(w, "  %-10s %s\n", s.Name, s.Desc)
	}
	fmt.Fprintln(w, "\npalettes:")
	for _, p := range palettes {
		fmt.Fprintf(w, "  %-10s %s\n", p.Name, p.Desc)
	}
	fmt.Fprintln(w, "\nlogos (-logo NAME or -svg FILE, -logo-pos center|left|right|corner, -logo-size tiny|small|medium|large|huge):")
	for _, l := range logos() {
		fmt.Fprintf(w, "  %-12s %s\n", l.Name, l.Title)
	}
}

// parseStyles turns the -style flag into the styles to pick from.
func parseStyles(spec string) ([]Style, error) {
	if spec == "random" || spec == "" {
		return styles, nil
	}
	var pool []Style
	for _, name := range strings.Split(spec, ",") {
		s, ok := findStyle(strings.TrimSpace(name))
		if !ok {
			return nil, fmt.Errorf("unknown style %q (try -list)", name)
		}
		pool = append(pool, s)
	}
	return pool, nil
}

// parsePalettes turns the -palette, -color and -colors flags into the palettes to pick from.
// -colors wins over -color, which wins over -palette.
func parsePalettes(names, color, colors string) ([]Palette, error) {
	switch {
	case colors != "":
		p, err := multiPalette(colors)
		return []Palette{p}, err
	case color != "":
		p, err := customPalette(color)
		return []Palette{p}, err
	case names == "random":
		return palettes, nil
	}
	var pals []Palette
	for _, name := range strings.Split(names, ",") {
		p, ok := findPalette(strings.TrimSpace(name))
		if !ok {
			return nil, fmt.Errorf("unknown palette %q (try -list)", name)
		}
		pals = append(pals, p)
	}
	return pals, nil
}

func findStyle(name string) (Style, bool) {
	for _, s := range styles {
		if s.Name == name {
			return s, true
		}
	}
	return Style{}, false
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "emberwall:", err)
	os.Exit(1)
}

func defaultOut() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Pictures", "wallpapers", "emberwall")
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ---------- screen size ----------

func parseSize(s string) (int, int, error) {
	if s == "auto" || s == "" {
		w, h := screenSize()
		return w, h, nil
	}
	parts := strings.Split(strings.ToLower(s), "x")
	if len(parts) == 2 {
		w, e1 := strconv.Atoi(parts[0])
		h, e2 := strconv.Atoi(parts[1])
		if e1 == nil && e2 == nil && w >= 64 && h >= 64 && w <= 16384 && h <= 16384 {
			return w, h, nil
		}
	}
	return 0, 0, fmt.Errorf("bad -size %q, want e.g. 3840x2160", s)
}

func screenSize() (int, int) {
	re := regexp.MustCompile(`(\d{3,5})\s*x\s*(\d{3,5})`)
	pick := func(b []byte) (int, int, bool) {
		m := re.FindSubmatch(b)
		if m == nil {
			return 0, 0, false
		}
		w, _ := strconv.Atoi(string(m[1]))
		h, _ := strconv.Atoi(string(m[2]))
		return w, h, w > 0 && h > 0
	}
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output(); err == nil {
			if i := strings.Index(string(out), "Resolution:"); i >= 0 {
				if w, h, ok := pick(out[i:]); ok {
					return w, h
				}
			}
		}
	case "linux":
		// Hyprland: biggest monitor
		if out, err := exec.Command("hyprctl", "monitors", "-j").Output(); err == nil {
			var mons []struct {
				Width  int `json:"width"`
				Height int `json:"height"`
			}
			if json.Unmarshal(out, &mons) == nil {
				bw, bh := 0, 0
				for _, m := range mons {
					if m.Width*m.Height > bw*bh {
						bw, bh = m.Width, m.Height
					}
				}
				if bw > 0 {
					return bw, bh
				}
			}
		}
		// KDE / X11 fallback
		if out, err := exec.Command("xrandr", "--current").Output(); err == nil {
			if i := strings.Index(string(out), "current"); i >= 0 {
				if w, h, ok := pick(out[i:]); ok {
					return w, h
				}
			}
		}
	}
	return 3840, 2160
}

// ---------- set wallpaper ----------

func have(cmd string) bool { _, err := exec.LookPath(cmd); return err == nil }

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func setWallpaper(p string) error {
	abs, _ := filepath.Abs(p)
	switch runtime.GOOS {
	case "darwin":
		// current Space(s) only; macOS keeps a separate picture per Space
		return run("osascript", "-e",
			fmt.Sprintf(`tell application "System Events" to tell every desktop to set picture to POSIX file %q`, abs))
	case "linux":
		desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
		switch {
		case strings.Contains(desktop, "kde") && have("plasma-apply-wallpaperimage"):
			return run("plasma-apply-wallpaperimage", abs)
		case have("swww") && exec.Command("swww", "query").Run() == nil:
			return run("swww", "img", abs, "--transition-type", "grow")
		case have("hyprctl"):
			if err := run("hyprctl", "hyprpaper", "preload", abs); err != nil {
				return err
			}
			return run("hyprctl", "hyprpaper", "wallpaper", ","+abs)
		}
		return fmt.Errorf("no swww, hyprpaper or plasma-apply-wallpaperimage found")
	}
	return fmt.Errorf("unsupported OS %s", runtime.GOOS)
}
