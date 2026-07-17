import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import SelectionToolbar from "./SelectionToolbar.vue";
import type { Item } from "../types";

const item: Item = {
  id: 1,
  type: "text",
  text: "Message",
  filename: "",
  filesizeBytes: 0,
  contentUrl: "",
  downloadUrl: "",
  createdAtEpochMillis: 0,
  publicLinkActive: false,
  metadata: {
    width: 0,
    height: 0,
    duration: "",
    mime: "text/plain",
    thumbnailUrl: "",
    playbackUrl: "",
    playbackMime: "",
    hlsUrl: "",
    processing: false,
  },
};

describe("SelectionToolbar", () => {
  it("keeps actions disabled until an item is selected", async () => {
    const wrapper = mount(SelectionToolbar, {
      props: { items: [], allItems: [item] },
    });
    const buttons = wrapper.findAll("button");
    const selectAllButton = buttons[0];
    const clearButton = buttons[1];
    const downloadButton = buttons[2];
    const deleteButton = buttons[3];
    const closeButton = buttons[4];
    if (!selectAllButton || !clearButton || !downloadButton || !deleteButton)
      throw new Error("Selection toolbar buttons are missing");
    if (!closeButton)
      throw new Error("Selection toolbar close button is missing");

    expect(buttons).toHaveLength(5);
    expect((selectAllButton.element as HTMLButtonElement).disabled).toBe(false);
    expect((clearButton.element as HTMLButtonElement).disabled).toBe(true);
    expect((downloadButton.element as HTMLButtonElement).disabled).toBe(true);
    expect((deleteButton.element as HTMLButtonElement).disabled).toBe(true);
    expect(selectAllButton.attributes("aria-label")).toBe("Select all");
    expect(closeButton.attributes("aria-label")).toBe("Exit selection mode");

    await selectAllButton.trigger("click");
    await closeButton.trigger("click");

    expect(wrapper.emitted("selectAll")).toHaveLength(1);
    expect(wrapper.emitted("close")).toHaveLength(1);
  });
});
