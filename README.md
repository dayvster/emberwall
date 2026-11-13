# emberwall

Random procedural wallpapers from the command line. Black and orange by default, any palette you like.

![synthwave](docs/synthwave-1.png)

25 styles, from flat 2D patterns (topographic lines, dunes, flow fields, circuit boards, digital rain)
to raymarched 3D fractals (mandelbulb, Menger sponge, mandelbox). Every image is made from a seed,
so you can get one back exactly by passing the same style, seed and size.

Standard library Go only. No dependencies, no network.

## Install

```sh
go install github.com/dayvster/emberwall@latest
```

or clone and `go build`.

## Usage

```sh
emberwall                          # one random wallpaper at your screen size
emberwall -n 12                    # a dozen of them
emberwall -style topo,flow -n 6    # only these styles
emberwall -style topo -seed 42     # remake one you liked (style and seed are in the file name)
emberwall -palette ice             # another palette, or -palette random
emberwall -color '#3fa9f5'         # your own colour (1-3 hex colours)
emberwall -colors '#bd93f9,#ff79c6,#8be9fd,#50fa7b'   # several accents blended across the image
emberwall -logo arch               # a distro logo on top of any style
emberwall -logo random -logo-pos corner   # small logo bottom-right (center, left, right, corner)
emberwall -logo arch -logo-size large      # tiny, small, medium (default), large/big or huge
emberwall -logo arch -logo-color '#1793d1' # logo in an exact colour instead of the palette's
emberwall -logo random -logo-color original   # each distro in its own brand colour
emberwall -set                     # also set the result as your wallpaper
emberwall -list                    # all styles, palettes and logos
```

Images go to `~/Pictures/wallpapers/emberwall` unless you pass `-o`. `-size WIDTHxHEIGHT` overrides
the detected screen size, `-grain 0` turns off film grain and `-crt` adds an old curved-monitor look.

### Setting the wallpaper

`-set` works with:

- macOS (System Events)
- KDE Plasma (`plasma-apply-wallpaperimage`)
- swww
- hyprpaper

Screen size is detected with `hyprctl monitors` on Hyprland, then `xrandr`, and falls back to 3840x2160.

## Styles

topo, synthwave, dunes, eclipse, flow, ridges, halftone, embers, matrix, lava, circuit, truchet,
lowpoly, nebula, skyline, planet, wireframe, orbits, crt, digital, fractal, mandelbulb, bulbwide,
menger, mandelbox

## Palettes

ember, ice, rose, toxic, violet, blood, gold, teal, mono, sunset, aurora. Or any colours with
`-color` / `-colors`.

## Logos

`-logo` puts a distro logo over any style, coloured by the active palette: almalinux, alpinelinux,
archlinux, artixlinux, debian, elementary, endeavouros, fedora, gentoo, kalilinux, linuxmint,
manjaro, nixos, opensuse, popos, rockylinux, ubuntu, voidlinux, zorin. Short names like `arch`,
`mint`, `pop` or `void` work too.

The shapes come from [Simple Icons](https://simpleicons.org) (CC0). The logos are trademarks of
their projects; see [logos/NOTICE.md](logos/NOTICE.md).

## License

MIT
