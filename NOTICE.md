# Notices

What is drawn after other works, without their code. The licences of the software the builds link are
generated at each release, and packed with the app as THIRD_PARTY_NOTICES.md (`go run ./scripts/notices`). Celestine
itself is under LICENSE.

## Cursor-Smith

The carets after Back-man, the Shredder, the Portal, Evaporate, the letter pops, the lightning, the
fireworks, the anvil sparks, the typewriter, the pixel trail, stardust, the speed demon, hot-head, the torch,
the smear, the energy beam, the aurora, the CRT and the neon (`caretfx_*.go`) are this app's own drawing of
the effects of [Cursor-Smith](https://github.com/Sadsnake1/cursor-smith) by Sadsnake1: its timings,
shapes and palettes were read from its source and written again for this app's painter, each as a fixed
preset with nothing to set. Cursor-Smith is MIT licensed:

```
MIT License

Copyright (c) 2026 Sadsnake1

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

Its sounds, and its settings, are not part of this app.

## synth-ui

The sync Worker's admin panel (`cloudflare/sync/admin`) is drawn with [synth-ui](https://github.com/unixzii/synth-ui)
by Cyandev (`@synth-ui/core`, `@synth-ui/widgets`, `@synth-ui/backend`, built into the Worker with its WebAssembly).
`admin/fonts.js` adds glyphs to its two faces. synth-ui is MIT licensed:

```
MIT License

Copyright (c) 2026 Cyandev <unixzii@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## tinyfix-tr and Zemberek-NLP

The Turkish spell checker (`internal/spell`, #623) is a Go port of [tinyfix-tr](https://github.com/imhalid/tinyfix-tr)'s
engine: its lexicon, rules, known misspellings, sentence pass and corrector. Its morphology (`internal/spell/morph`) is
a port of tinyfix-tr's, whose TypeScript was adapted from [Zemberek-NLP](https://github.com/ahmetaa/zemberek-nlp),
Copyright 2018 Ahmet A. Akın, Mehmet D. Akın, under the Apache License, Version 2.0: what was adapted from which of its
files, and the licence's full text, are in `internal/spell/morph/NOTICE`. The spell checker's data is not in the app:
it is downloaded from [Hugging Face](https://huggingface.co/imhalid/tinyfix-tr) when it is turned on; its word counts,
from Turkish Wikipedia and OpenSubtitles, are under CC BY-SA 4.0, as the settings say beside it. tinyfix-tr is MIT
licensed:

```
MIT License

Copyright (c) 2026 imhalid

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
