// Original 5 x 7 bitmap alphabet. Generate the bundled TrueType font using
// only the repository's Node runtime: ./node scripts/build-pixel-font.mjs.
import fs from 'node:fs';

const alphabet = {
  A: ['01110','10001','10001','11111','10001','10001','10001'],
  B: ['11110','10001','10001','11110','10001','10001','11110'],
  C: ['01111','10000','10000','10000','10000','10000','01111'],
  D: ['11110','10001','10001','10001','10001','10001','11110'],
  E: ['11111','10000','10000','11110','10000','10000','11111'],
  F: ['11111','10000','10000','11110','10000','10000','10000'],
  G: ['01111','10000','10000','10111','10001','10001','01111'],
  H: ['10001','10001','10001','11111','10001','10001','10001'],
  I: ['11111','00100','00100','00100','00100','00100','11111'],
  J: ['00111','00010','00010','00010','10010','10010','01100'],
  K: ['10001','10010','10100','11000','10100','10010','10001'],
  L: ['10000','10000','10000','10000','10000','10000','11111'],
  M: ['10001','11011','10101','10101','10001','10001','10001'],
  N: ['10001','11001','11001','10101','10011','10011','10001'],
  O: ['01110','10001','10001','10001','10001','10001','01110'],
  P: ['11110','10001','10001','11110','10000','10000','10000'],
  Q: ['01110','10001','10001','10001','10101','10010','01101'],
  R: ['11110','10001','10001','11110','10100','10010','10001'],
  S: ['01111','10000','10000','01110','00001','00001','11110'],
  T: ['11111','00100','00100','00100','00100','00100','00100'],
  U: ['10001','10001','10001','10001','10001','10001','01110'],
  V: ['10001','10001','10001','10001','10001','01010','00100'],
  W: ['10001','10001','10001','10101','10101','11011','10001'],
  X: ['10001','10001','01010','00100','01010','10001','10001'],
  Y: ['10001','10001','01010','00100','00100','00100','00100'],
  Z: ['11111','00001','00010','00100','01000','10000','11111'],
  '0': ['01110','10001','10011','10101','11001','10001','01110'],
  '1': ['00100','01100','00100','00100','00100','00100','01110'],
  '2': ['01110','10001','00001','00010','00100','01000','11111'],
  '3': ['11110','00001','00001','01110','00001','00001','11110'],
  '4': ['00010','00110','01010','10010','11111','00010','00010'],
  '5': ['11111','10000','10000','11110','00001','00001','11110'],
  '6': ['01110','10000','10000','11110','10001','10001','01110'],
  '7': ['11111','00001','00010','00100','01000','01000','01000'],
  '8': ['01110','10001','10001','01110','10001','10001','01110'],
  '9': ['01110','10001','10001','01111','00001','00001','01110'],
  ' ': [], '.': ['00000','00000','00000','00000','00000','00110','00110'],
  ',': ['00000','00000','00000','00000','00110','00100','01000'],
  ':': ['00000','00110','00110','00000','00110','00110','00000'],
  ';': ['00000','00110','00110','00000','00110','00100','01000'],
  '!': ['00100','00100','00100','00100','00100','00000','00100'],
  '?': ['01110','10001','00001','00010','00100','00000','00100'],
  '-': ['00000','00000','00000','11111','00000','00000','00000'],
  '_': ['00000','00000','00000','00000','00000','00000','11111'],
  '+': ['00000','00100','00100','11111','00100','00100','00000'],
  '=': ['00000','00000','11111','00000','11111','00000','00000'],
  '/': ['00001','00001','00010','00100','01000','10000','10000'],
  '\\': ['10000','10000','01000','00100','00010','00001','00001'],
  '(': ['00010','00100','01000','01000','01000','00100','00010'],
  ')': ['01000','00100','00010','00010','00010','00100','01000'],
  '[': ['01110','01000','01000','01000','01000','01000','01110'],
  ']': ['01110','00010','00010','00010','00010','00010','01110'],
  '<': ['00001','00010','00100','01000','00100','00010','00001'],
  '>': ['10000','01000','00100','00010','00100','01000','10000'],
  "'": ['00100','00100','00000','00000','00000','00000','00000'],
  '"': ['01010','01010','00000','00000','00000','00000','00000'],
  '#': ['01010','01010','11111','01010','11111','01010','01010'],
  '@': ['01110','10001','10111','10101','10111','10000','01111'],
  '%': ['11001','11010','00010','00100','01000','01011','10011'],
  '*': ['00000','10101','01110','11111','01110','10101','00000'],
  '|': ['00100','00100','00100','00100','00100','00100','00100'],
  '&': ['01100','10010','10100','01000','10101','10010','01101'],
  '$': ['00100','01111','10100','01110','00101','11110','00100'],
  '^': ['00100','01010','10001','00000','00000','00000','00000'],
  '`': ['01000','00100','00000','00000','00000','00000','00000'],
  '~': ['00000','00000','01001','10110','00000','00000','00000'],
};
alphabet['{'] = alphabet['[']; alphabet['}'] = alphabet[']'];
const u16 = value => { const b = Buffer.alloc(2); b.writeUInt16BE(value & 0xffff); return b; };
const u32 = value => { const b = Buffer.alloc(4); b.writeUInt32BE(value >>> 0); return b; };
const concat = values => Buffer.concat(values);
const pad = buffer => concat([buffer, Buffer.alloc((4 - buffer.length % 4) % 4)]);

function glyph(rows) {
  const contours = [];
  rows.forEach((row, y) => {
    for (let x = 0; x < row.length; x++) {
      if (row[x] !== '1') continue;
      const start = x;
      while (row[x + 1] === '1') x++;
      const left = start * 100, right = (x + 1) * 100, bottom = (rows.length - 1 - y) * 100;
      contours.push([[left,bottom],[left,bottom+100],[right,bottom+100],[right,bottom]]);
    }
  });
  if (!contours.length) return Buffer.alloc(0);
  const points = contours.flat();
  const xs = [], ys = [];
  let previousX = 0, previousY = 0;
  for (const [x,y] of points) { xs.push(u16(x - previousX)); ys.push(u16(y - previousY)); previousX = x; previousY = y; }
  return pad(concat([u16(contours.length), u16(0), u16(Math.min(0, ...points.map(p => p[1]))), u16(500), u16(Math.max(700, ...points.map(p => p[1]))), ...contours.map((_,i) => u16(i*4+3)), u16(0), Buffer.alloc(points.length, 1), ...xs, ...ys]));
}

const entries = [[0, alphabet['?']]];
for (let code = 32; code <= 126; code++) entries.push([code, alphabet[String.fromCharCode(code).toUpperCase()] || alphabet['?']]);
// Latin accents remain recognisable in player names and server descriptions.
for (let code = 160; code <= 383; code++) {
  const letter = String.fromCharCode(code), normalized = letter.normalize('NFD');
  const base = alphabet[normalized[0].toUpperCase()];
  if (!base) continue;
  let rows = [...base];
  if (normalized.length > 1) {
    const accents = {'\u0300':['01000','00100'], '\u0301':['00010','00100'], '\u0302':['00100','01010'], '\u0303':['01001','10110'], '\u0308':['01010','00000'], '\u030a':['00100','01010']};
    if (accents[normalized[1]]) rows = [...accents[normalized[1]], '00000', ...rows];
    if (normalized[1] === '\u0327') rows = [...rows, '00100', '01000'];
  }
  entries.push([code, rows]);
}
for (const [code, drawing] of [[0x2013,'-'],[0x2014,'-'],[0x2018,"'"],[0x2019,"'"],[0x201c,'"'],[0x201d,'"'],[0x2026,'.'],[0x00b7,'.']]) {
  if (!entries.some(entry => entry[0] === code)) entries.push([code, alphabet[drawing]]);
}
entries.sort((a,b) => a[0]-b[0]);
const glyphs = entries.map(([,rows]) => glyph(rows));
const count = glyphs.length;
const tables = {};
tables.glyf = concat(glyphs);
let offset = 0;
tables.loca = concat([...glyphs.map(g => { const result = u32(offset); offset += g.length; return result; }), u32(offset)]);
tables.hmtx = concat(entries.map(() => concat([u16(600),u16(0)])));
tables.head = Buffer.alloc(54);
tables.head.writeUInt32BE(0x00010000,0); tables.head.writeUInt32BE(0x00010000,4);
tables.head.writeUInt32BE(0x5f0f3cf5,12); tables.head.writeUInt16BE(3,16); tables.head.writeUInt16BE(1000,18);
tables.head.writeInt16BE(-200,38); tables.head.writeInt16BE(500,40); tables.head.writeInt16BE(1000,42);
tables.head.writeUInt16BE(8,46); tables.head.writeInt16BE(2,48); tables.head.writeInt16BE(1,50);
tables.hhea = Buffer.alloc(36);
tables.hhea.writeUInt32BE(0x00010000,0); tables.hhea.writeInt16BE(1000,4); tables.hhea.writeInt16BE(-200,6);
tables.hhea.writeUInt16BE(600,10); tables.hhea.writeInt16BE(500,16); tables.hhea.writeInt16BE(1,18); tables.hhea.writeUInt16BE(count,34);
tables.maxp = concat([u32(0x00010000),u16(count),u16(200),u16(50),u16(0),u16(0),u16(2),...Array.from({length:8}, () => u16(0))]);
const codes = entries.slice(1).map(([code],index) => ({code,id:index+1}));
const segments = [...codes, {code:0xffff,id:0}];
const segCount = segments.length, power = 2 ** Math.floor(Math.log2(segCount));
const format4 = concat([u16(4),u16(16+segCount*8),u16(0),u16(segCount*2),u16(power*2),u16(Math.log2(power)),u16(segCount*2-power*2), ...segments.map(s => u16(s.code)),u16(0),...segments.map(s => u16(s.code)),...segments.map(s => u16(s.id-s.code)),...segments.map(() => u16(0))]);
tables.cmap = concat([u16(0),u16(1),u16(3),u16(1),u32(12),format4]);
const names = [[1,'TAP Pixel'],[2,'Regular'],[3,'TAP-Pixel-1'],[4,'TAP Pixel'],[5,'Version 1.0'],[6,'TAP-Pixel']];
const nameStrings = names.map(([,str]) => concat([...str].map(c => u16(c.charCodeAt(0)))));
let nameOffset = 0;
tables.name = concat([u16(0),u16(names.length),u16(6+names.length*12),...names.map(([id],i) => { const record = concat([u16(3),u16(1),u16(0x409),u16(id),u16(nameStrings[i].length),u16(nameOffset)]); nameOffset += nameStrings[i].length; return record; }), ...nameStrings]);
tables.post = Buffer.alloc(32); tables.post.writeUInt32BE(0x00030000,0); tables.post.writeUInt32BE(1,12);
tables['OS/2'] = Buffer.alloc(78);
tables['OS/2'].writeInt16BE(600,2); tables['OS/2'].writeUInt16BE(400,4); tables['OS/2'].writeUInt16BE(5,6);
tables['OS/2'].writeUInt32BE(3,42); tables['OS/2'].write('TAP ',58);
tables['OS/2'].writeUInt16BE(0x40,62); tables['OS/2'].writeUInt16BE(32,64); tables['OS/2'].writeUInt16BE(0x2026,66);
tables['OS/2'].writeInt16BE(1000,68); tables['OS/2'].writeInt16BE(-200,70); tables['OS/2'].writeUInt16BE(1000,74); tables['OS/2'].writeUInt16BE(200,76);
const checksum = buffer => { let sum = 0; const bytes = pad(buffer); for (let i=0;i<bytes.length;i+=4) sum = (sum + bytes.readUInt32BE(i)) >>> 0; return sum; };
const tags = Object.keys(tables).sort(), tablePower = 2 ** Math.floor(Math.log2(tags.length));
offset = 12 + tags.length*16;
let headOffset;
const directory = tags.map(tag => { const value = tables[tag]; if (tag === 'head') headOffset = offset; const record = concat([Buffer.from(tag),u32(checksum(value)),u32(offset),u32(value.length)]); offset += pad(value).length; return record; });
const font = concat([u32(0x00010000),u16(tags.length),u16(tablePower*16),u16(Math.log2(tablePower)),u16(tags.length*16-tablePower*16),...directory,...tags.map(tag => pad(tables[tag]))]);
font.writeUInt32BE((0xb1b0afba-checksum(font)) >>> 0,headOffset+8);
const target = new URL('../Project/client-gui/frontend/public/fonts/tap-pixel.ttf', import.meta.url);
fs.mkdirSync(new URL('.',target),{recursive:true});
fs.writeFileSync(target,font);
console.log(`TAP Pixel: ${count} glyphs, ${font.length} bytes.`);
