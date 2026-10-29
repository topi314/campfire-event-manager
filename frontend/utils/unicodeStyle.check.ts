import assert from "node:assert/strict";
import {
  applyLetterStyle,
  autocompleteRange,
  findStyledTokens,
  foldPlain,
  toggleFormat,
} from "./unicodeStyle.ts";

const bold = toggleFormat("Community Day", "bold");
assert.equal(bold, "𝐂𝐨𝐦𝐦𝐮𝐧𝐢𝐭𝐲 𝐃𝐚𝐲");
assert.equal(foldPlain(bold), "Community Day");
assert.equal(toggleFormat(bold, "bold"), "Community Day");

const italic = toggleFormat("hi", "italic");
assert.equal(foldPlain(italic), "hi");
assert.ok(italic.includes("\u210e"));

const both = toggleFormat(toggleFormat("Go", "bold"), "italic");
assert.equal(foldPlain(both), "Go");
assert.equal(toggleFormat(both, "italic"), toggleFormat("Go", "bold"));

assert.equal(toggleFormat("Meet", "mono"), "𝙼𝚎𝚎𝚝");

const under = toggleFormat("Meet", "underline");
assert.equal(under, "M̲e̲e̲t̲");
assert.equal(toggleFormat(under, "underline"), "Meet");
const strike = toggleFormat("Meet", "strike");
assert.equal(strike, "M̶e̶e̶t̶");

const styledKey = "{{" + toggleFormat("club", "bold") + "}}";
const tokens = findStyledTokens(`Hello ${styledKey}!`);
assert.equal(tokens.length, 1);
assert.equal(tokens[0].key, "club");
assert.equal(tokens[0].style, "bold");
assert.equal(applyLetterStyle("PoGo", tokens[0].style, false, false), "𝐏𝐨𝐆𝐨");

const underKey = "{{" + toggleFormat("city", "underline") + "}}";
const underTok = findStyledTokens(underKey)[0];
assert.equal(underTok.key, "city");
assert.equal(underTok.underline, true);
assert.equal(applyLetterStyle("Berlin", "plain", true, false), "B̲e̲r̲l̲i̲n̲");

const open = autocompleteRange("See {{cl", "See {{cl".length);
assert.deepEqual(open, { start: 4, end: "See {{cl".length, query: "cl" });

const inside = "Title {{club}}";
const mid = autocompleteRange(inside, inside.indexOf("club") + 2);
assert.equal(mid?.query, "cl");
assert.equal(mid?.start, inside.indexOf("{{"));
assert.equal(mid?.end, inside.length);

console.log("unicodeStyle ok");
