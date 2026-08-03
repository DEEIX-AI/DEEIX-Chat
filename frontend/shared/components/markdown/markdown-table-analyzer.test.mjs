import assert from "node:assert/strict";
import test from "node:test";

import {
  classifyColumn,
  classifyTableColumns,
  createColumnAnalyzerConfig,
  DEFAULT_COLUMN_ANALYZER_PATTERNS,
  getVisualLength,
  mergeColumnType,
} from "./markdown-table-analyzer.ts";

test("classifies a mixed table from values rather than column position", () => {
  const headers = ["编号", "Name", "説明", "Date"];
  const rows = [
    ["1", "Alpha", "这是一个较长的中文说明，用于解释问题发生的原因以及推荐的处理方式。", "2026-03-20"],
    ["2", "ベータ", "This description contains enough natural-language detail to remain readable on a phone.", "2026-03-21"],
    ["3", "Gamma", "複数の言語を含む長い説明で、列が狭くなりすぎないことを確認します。", "2026-03-22"],
  ];

  assert.deepEqual(classifyTableColumns(headers, rows), ["numeric", "normal", "content", "date"]);
});

test("counts CJK and other wide characters as two visual units", () => {
  assert.equal(getVisualLength("ab中文"), 6);
  assert.equal(getVisualLength("テスト"), 6);
});

test("classifies multiple long natural-language columns as content", () => {
  const types = classifyTableColumns(
    ["原因", "Recommendation", "詳細"],
    [
      [
        "因为移动端将所有列压缩到屏幕内，长文本被拆成了大量短行。",
        "Allow the table to grow naturally and keep horizontal scrolling inside its container.",
        "ユーザーが内容を読みやすいように、長文列には十分な最小幅を確保します。",
      ],
      [
        "第二个原因字段继续包含完整的自然语言句子，以提供稳定的统计样本。",
        "Use column-level metadata instead of fixed child indexes for unknown generated schemas.",
        "ストリーミング中は狭い列から広い列へのアップグレードだけを許可します。",
      ],
    ],
  );

  assert.deepEqual(types, ["content", "content", "content"]);
});

test("recognizes all-numeric values including money and percentages", () => {
  assert.deepEqual(
    classifyTableColumns(
      ["Count", "金额", "Ratio"],
      [
        ["1,200", "￥99.50", "12%"],
        ["3,400", "￥125.00", "8.5%"],
        ["5,600", "￥1,020.25", "100%"],
      ],
    ),
    ["numeric", "numeric", "numeric"],
  );
});

test("keeps long structured-column fallback text wrap-safe", () => {
  assert.equal(
    classifyColumn("Amount", [
      "1",
      "2",
      "3",
      "This value could not be calculated because the upstream response omitted pricing metadata.",
    ]),
    "normal",
  );
  assert.equal(
    classifyColumn("Date", [
      "2026-03-20",
      "2026-03-21",
      "2026-03-22",
      "The date is unavailable because this record predates the migration.",
    ]),
    "normal",
  );
  assert.equal(classifyColumn("Amount", ["1", "2", "3", "N/A"]), "numeric");
});

test("classifies URLs and long unbroken identifiers as code", () => {
  assert.deepEqual(
    classifyTableColumns(
      ["未知字段", "Token"],
      [
        ["https://example.com/a/very/long/path?query=mobile-table", "customer_session_identifier_01HZX4RTN8Q3YJ7K9MP2W6CVBX"],
        ["/srv/app/releases/2026-03-20/config.json", "customer_session_identifier_01HZX4RTN8Q3YJ7K9MP2W6CVBY"],
      ],
    ),
    ["code", "code"],
  );
});

test("supports multilingual and unknown headers without relying on hints", () => {
  assert.deepEqual(
    classifyTableColumns(
      ["名称", "Category", "更新日時", "Champ inconnu"],
      [
        ["Alpha", "Tool", "2026-03-20 09:30", "Medium value"],
        ["ベータ", "Library", "2026-03-21 10:45", "Another value"],
        ["Gamma", "Service", "2026-03-22 11:15", "Third value"],
      ],
      { headerRules: false },
    ),
    ["normal", "normal", "date", "normal"],
  );
});

test("ignores empty and missing cells while preserving uneven columns", () => {
  assert.deepEqual(
    classifyTableColumns(
      ["ID", "Name", "Notes"],
      [
        ["1", "Alpha Product"],
        ["2", "", ""],
        ["3", "Gamma Service", "This populated note is long enough to be handled as readable natural-language content."],
        [],
      ],
    ),
    ["numeric", "normal", "content"],
  );
});

test("header rules remain weak, replaceable hints", () => {
  const shortDescription = ["ok", "new", "done", "hold"];
  assert.notEqual(classifyColumn("Description", shortDescription), "content");
  assert.equal(
    classifyColumn("Arbitrary", ["x", "y", "z"], {
      headerRules: [{ pattern: /^Arbitrary$/, type: "date", weight: 0.2 }],
    }),
    "compact",
  );
});
test("normalizes complete pattern overrides with partial thresholds", () => {
  const patterns = Object.fromEntries(
    Object.entries(DEFAULT_COLUMN_ANALYZER_PATTERNS).map(([name, pattern]) => [name, new RegExp(pattern.source, pattern.flags)]),
  );
  assert.equal(
    classifyColumn("Value", ["1", "2", "3"], {
      patterns,
      thresholds: { numericRatio: 0.5 },
    }),
    "numeric",
  );
  assert.equal(classifyColumn("Value", ["1", "2", "3"], createColumnAnalyzerConfig()), "numeric");
});

test("treats whitespace-free CJK prose as natural language rather than code", () => {
  assert.equal(
    classifyColumn("未知", [
      "这是没有空格但依然属于自然语言的中文长文本内容",
      "これは空白がなくても自然言語として扱うべき日本語の長文です",
    ]),
    "content",
  );
});

test("upgrades columns without shrinking a previous streaming type", () => {
  const initial = classifyColumn("Value", ["Alpha Product", "Beta Service", "Gamma Library"]);
  const expanded = classifyColumn("Value", [
    "Alpha now includes a complete explanation with substantially more natural-language detail.",
    "Beta now includes a second detailed sentence because the streamed answer continued growing.",
    "Gamma now also contains enough descriptive prose to need a readable content-column width.",
  ]);

  assert.equal(initial, "normal");
  assert.equal(expanded, "content");
  assert.equal(mergeColumnType(initial, expanded), "content");
  assert.equal(mergeColumnType("content", "normal"), "content");
});
