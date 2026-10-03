# Third-party notices

rabbitrun is released under the [MIT License](./LICENSE). The following parts are covered by their own licenses.

## Fonts

| File | Font | License |
| ---- | ---- | ------- |
| `assets/fonts/LilitaOne-Regular.ttf` | Lilita One by Juan Montoreano | [SIL Open Font License 1.1](./assets/fonts/OFL-LilitaOne.txt) |
| `assets/fonts/mplus-1p-regular.ttf` | M+ 1p Regular by M+ FONTS PROJECT | [M+ FONTS License](./assets/fonts/LICENSE-MPLUS.txt) (free to use, copy and distribute, with or without modification) |

## Music

The background music is original drum'n'bass arrangements synthesized at run time (`music.go`). The melodies and chords come from these scores:

| Song | Source | License |
| ---- | ------ | ------- |
| Vivaldi, *La Primavera (Spring)* Op. 8 No. 1, 1st movement | [Mutopia Project piece 301](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=301) | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |
| Vivaldi, *L'Estate (Summer)* Op. 8 No. 2, 1st movement | [Mutopia Project piece 336](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=336) | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |
| Vivaldi, *L'Autunno (Autumn)* Op. 8 No. 3, 1st movement | [Mutopia Project piece 350](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=350) | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |
| Vivaldi, *L'Inverno (Winter)* Op. 8 No. 4, 1st movement | [Mutopia Project piece 351](https://www.mutopiaproject.org/cgibin/piece-info.cgi?id=351) | [CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/) |

The note data for the *Four Seasons* in `songs_data.go` (`songSpring`, `songSummer`, `songAutumn`, `songWinter`) are adaptations of the Mutopia Project scores and are shared under the same CC BY-SA 3.0 license.

## Go modules

The licenses of the Go modules rabbitrun depends on are checked in CI (`.github/workflows/licenses.yml`) and listed by `go-licenses report github.com/nao1215/rabbitrun`.
