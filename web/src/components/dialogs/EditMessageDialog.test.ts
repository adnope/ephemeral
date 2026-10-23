import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../../api";
import { publishRealtime } from "../../state/realtime";
import { ui } from "../../state/ui";
import type { Item } from "../../types";
import { formatDateTime } from "../../utils/format";
import ItemCard from "../items/ItemCard.vue";
import EditMessageDialog from "./EditMessageDialog.vue";

vi.mock("../../api", () => ({ api: { editMessage: vi.fn() } }));
vi.mock("../../state/realtime", () => ({ publishRealtime: vi.fn() }));

const item: Item = {
  id: 42,
  type: "text",
  text: "Original text\nSecond line",
  filename: "",
  filesizeBytes: 0,
  contentUrl: "",
  downloadUrl: "",
  createdAtEpochMillis: 1710000000000,
  editedAtEpochMillis: null,
  publicLinkActive: false,
  metadata: {
    width: 0,
    height: 0,
    duration: "",
    mime: "",
    thumbnailUrl: "",
    playbackUrl: "",
    playbackMime: "",
    hlsUrl: "",
    processing: false,
  },
};

const wrappers: Array<{ unmount: () => void }> = [];

afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount();
  ui.editItem = null;
  vi.resetAllMocks();
});

function mountDialog() {
  const wrapper = mount(EditMessageDialog, {
    props: { item },
    global: { stubs: { teleport: true } },
  });
  wrappers.push(wrapper);
  return wrapper;
}

function mountCard(value = item) {
  const wrapper = mount(ItemCard, { props: { item: value } });
  wrappers.push(wrapper);
  return wrapper;
}

describe("EditMessageDialog", () => {
  it("starts with the original content and cancels without saving", async () => {
    const wrapper = mountDialog();
    expect(wrapper.get("textarea").element.value).toBe(item.text);
    await wrapper.get("textarea").setValue("Unsaved changes");
    await wrapper.get(".modal-footer button").trigger("click");
    expect(wrapper.emitted("close")).toHaveLength(1);
    expect(api.editMessage).not.toHaveBeenCalled();
    expect(item.text).toBe("Original text\nSecond line");
  });

  it("saves changed content, updates the feed, and closes", async () => {
    const edited = {
      ...item,
      text: "Updated text",
      editedAtEpochMillis: 1710001000000,
    };
    vi.mocked(api.editMessage).mockResolvedValueOnce(edited);
    const wrapper = mountDialog();
    await wrapper.get("textarea").setValue("  Updated text\n");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(api.editMessage).toHaveBeenCalledWith(item.id, "Updated text");
    expect(publishRealtime).toHaveBeenCalledWith({
      type: "item:result",
      id: item.id,
      item: edited,
    });
    expect(wrapper.emitted("close")).toHaveLength(1);
  });

  it("rejects blank text without sending a request", async () => {
    const wrapper = mountDialog();
    await wrapper.get("textarea").setValue(" \n\t ");
    expect(
      wrapper.get('button[type="submit"]').attributes("disabled"),
    ).toBeDefined();
    await wrapper.get("form").trigger("submit");
    expect(wrapper.get('[role="alert"]').text()).toBe("Text must not be empty");
    expect(api.editMessage).not.toHaveBeenCalled();
  });

  it("keeps the draft after errors and allows retrying", async () => {
    vi.mocked(api.editMessage).mockRejectedValueOnce(new Error("Edit failed"));
    const wrapper = mountDialog();
    await wrapper.get("textarea").setValue("My draft");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toBe("Edit failed");
    expect(wrapper.get("textarea").element.value).toBe("My draft");
    expect(wrapper.emitted("close")).toBeUndefined();
    expect(publishRealtime).not.toHaveBeenCalled();
    vi.mocked(api.editMessage).mockResolvedValueOnce({
      ...item,
      text: "My draft",
    });
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.emitted("close")).toHaveLength(1);
  });

  it("prevents duplicate saves and dismissal while saving", async () => {
    let resolve!: (value: Item) => void;
    vi.mocked(api.editMessage).mockReturnValue(
      new Promise<Item>((done) => {
        resolve = done;
      }),
    );
    const wrapper = mountDialog();
    await wrapper.get("form").trigger("submit");
    await wrapper.get("form").trigger("submit");
    await wrapper.get('[aria-label="Close"]').trigger("click");
    expect(api.editMessage).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted("close")).toBeUndefined();
    expect(wrapper.get("textarea").element.disabled).toBe(true);
    expect(
      wrapper.get(".modal-footer button").attributes("disabled"),
    ).toBeDefined();
    resolve(item);
    await flushPromises();
  });
});

describe("ItemCard editing", () => {
  it("opens editing for text items from the action menu", async () => {
    const wrapper = mountCard();
    await wrapper.get('[aria-label="Item actions"]').trigger("click");
    const edit = wrapper
      .findAll('[role="menuitem"]')
      .find((entry) => entry.text() === "Edit");
    expect(edit).toBeDefined();
    await edit!.trigger("click");
    expect(ui.editItem?.id).toBe(item.id);
    expect(wrapper.find('[role="menu"]').exists()).toBe(false);
  });

  it.each(["file", "image", "video"] as const)(
    "does not offer editing for %s items",
    async (type) => {
      const wrapper = mountCard({ ...item, type });
      await wrapper.get('[aria-label="Item actions"]').trigger("click");
      expect(
        wrapper
          .findAll('[role="menuitem"]')
          .some((entry) => entry.text() === "Edit"),
      ).toBe(false);
    },
  );

  it("shows the last edited timestamp only after an edit", async () => {
    const wrapper = mountCard();
    expect(wrapper.find(".item-last-edited").exists()).toBe(false);
    const editedAt = 1710001000000;
    await wrapper.setProps({
      item: { ...item, editedAtEpochMillis: editedAt },
    });
    expect(wrapper.get(".item-last-edited").text()).toContain("Last edited");
    expect(wrapper.get(".item-last-edited").text()).toContain(
      formatDateTime(editedAt),
    );
    expect(wrapper.get(".item-last-edited time").attributes("datetime")).toBe(
      new Date(editedAt).toISOString(),
    );
    expect(wrapper.get(".item-timestamps > time").attributes("datetime")).toBe(
      new Date(item.createdAtEpochMillis).toISOString(),
    );
  });
});
