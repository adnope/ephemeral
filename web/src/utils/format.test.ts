import { describe, expect, it } from "vitest";
import { formatBytes, linkify } from "./format";

describe("formatBytes", () => {
  it("formats byte and binary-scaled values", () => {
    expect(formatBytes(13)).toBe("13 B");
    expect(formatBytes(1536)).toBe("1.5 KB");
  });
});

describe("linkify", () => {
  it("keeps punctuation outside links", () => {
    expect(linkify("See example.com/docs.")).toEqual([
      { text: "See " },
      { text: "example.com/docs", href: "https://example.com/docs" },
      { text: "." },
    ]);
  });

  it("does not turn arbitrary text into links", () => {
    expect(linkify("plain message")).toEqual([{ text: "plain message" }]);
  });
});
