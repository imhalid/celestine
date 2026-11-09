// The faces of synth-ui with the letters the admin panel needs and they lack: Turkish's own (Ç Ğ İ Ö Ş Ü, and
// their lowercase, shown as capitals as the faces show every letter), the @ of every e-mail address, and a few
// marks. A face is data: these are its glyphs and more, drawn on the same grids (5×7, and the small face's 5 rows,
// where the dots and hooks take a row of the letter's own).
import { FONT_5X7, FONT_SMALL } from '@synth-ui/widgets';

// Both faces get a blank row above and one below every glyph (pad): the dots and hooks of a Turkish letter go there,
// and its body is as tall as the others'.
const TEXT = {
  Ç: '..... .###. #...# #.... #.... #.... #...# .###. ..#..',
  Ğ: '.###. .###. #...# #.... #.### #...# #...# .#### .....',
  İ: '..#.. .###. ..#.. ..#.. ..#.. ..#.. ..#.. .###. .....',
  Ö: '.#.#. .###. #...# #...# #...# #...# #...# .###. .....',
  Ş: '..... .#### #.... #.... .###. ....# ....# ####. ..#..',
  Ü: '.#.#. #...# #...# #...# #...# #...# #...# .###. .....',
  '@': '..... .###. #...# #.### #.#.# #.### #.... .#### .....',
  '—': '..... ..... ..... ..... ##### ..... ..... ..... .....',
  '–': '..... ..... ..... ..... .###. ..... ..... ..... .....',
  '▸': '..... #.... ##... ###.. ####. ###.. ##... #.... .....',
  '✓': '..... ..... ....# ...#. #.#.. .#... ..... ..... .....',
  '↑': '..... ..#.. .###. #.#.# ..#.. ..#.. ..#.. ..#.. .....',
  '↓': '..... ..#.. ..#.. ..#.. ..#.. #.#.# .###. ..#.. .....',
  '…': '..... ..... ..... ..... ..... ..... ..... #.#.# .....',
  '•': '..... ..... ..... .###. .###. .###. ..... ..... .....',
};

const SMALL = {
  Ç: '.... .### #... #... #... .### ..#.',
  Ğ: '.##. .### #... #.## #..# .### ....',
  İ: '.#. ### .#. .#. .#. ### ...',
  Ö: '#..# .##. #..# #..# #..# .##. ....',
  Ş: '.... .### #... .##. ...# ###. .#..',
  Ü: '#..# #..# #..# #..# #..# .##. ....',
  '@': '.... .##. #..# #.## #... .### ....',
  '—': '.... .... .... #### .... .... ....',
  '–': '... ... ... ### ... ... ...',
  '▸': '... #.. ##. ### ##. #.. ...',
  '✓': '.... .... ...# #.#. .#.. .... ....',
  '↑': '... .#. ### .#. .#. .#. ...',
  '↓': '... .#. .#. .#. ### .#. ...',
  '•': '.. .. ## ## .. .. ..',
};

/** pad puts a blank row above and below every glyph of a face. */
const pad = (glyphs) =>
  Object.fromEntries(
    Object.entries(glyphs).map(([ch, rows]) => {
      const blank = '.'.repeat(rows.split(' ')[0].length);
      return [ch, `${blank} ${rows} ${blank}`];
    }),
  );

/** The 5×7 face with the letters added. */
export const TEXT_FACE = { ...FONT_5X7, glyphs: { ...pad(FONT_5X7.glyphs), ...TEXT } };
/** The small face with the letters added. */
export const SMALL_FACE = { ...FONT_SMALL, glyphs: { ...pad(FONT_SMALL.glyphs), ...SMALL } };
/** The faces, by the names the widgets ask for. */
export const FONTS = { text: TEXT_FACE, small: SMALL_FACE };
