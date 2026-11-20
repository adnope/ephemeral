import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../../api";
import ChangePasswordDialog from "./ChangePasswordDialog.vue";

vi.mock("../../api", () => ({ api: { changePassword: vi.fn() } }));

afterEach(() => vi.resetAllMocks());

function mountDialog() {
  return mount(ChangePasswordDialog, {
    global: { stubs: { teleport: true } },
  });
}

async function fill(
  wrapper: ReturnType<typeof mountDialog>,
  confirmation = "new password",
) {
  const inputs = wrapper.findAll("input");
  await inputs[0]!.setValue("old password");
  await inputs[1]!.setValue("new password");
  await inputs[2]!.setValue(confirmation);
}

describe("ChangePasswordDialog", () => {
  it("rejects mismatched confirmation before sending a request", async () => {
    const wrapper = mountDialog();
    await fill(wrapper, "different");
    await wrapper.get("form").trigger("submit");
    expect(wrapper.get('[role="alert"]').text()).toBe(
      "New passwords do not match",
    );
    expect(api.changePassword).not.toHaveBeenCalled();
    wrapper.unmount();
  });

  it("shows backend errors and allows a successful retry", async () => {
    vi.mocked(api.changePassword).mockRejectedValueOnce(
      new Error("Old password is incorrect"),
    );
    const wrapper = mountDialog();
    await fill(wrapper);
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toBe(
      "Old password is incorrect",
    );
    expect(wrapper.emitted("close")).toBeUndefined();

    vi.mocked(api.changePassword).mockResolvedValueOnce(undefined);
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(api.changePassword).toHaveBeenLastCalledWith(
      "old password",
      "new password",
    );
    expect(wrapper.get('[role="status"]').text()).toBe(
      "Your password has been changed.",
    );
    expect(wrapper.findAll("input")).toHaveLength(0);
    expect(wrapper.find('[role="alert"]').exists()).toBe(false);
    await wrapper.get(".modal-footer button").trigger("click");
    expect(wrapper.emitted("close")).toHaveLength(1);
    wrapper.unmount();
  });

  it("prevents duplicate submissions and dismissal while saving", async () => {
    let resolve!: () => void;
    vi.mocked(api.changePassword).mockReturnValue(
      new Promise<void>((done) => {
        resolve = done;
      }),
    );
    const wrapper = mountDialog();
    await fill(wrapper);
    await wrapper.get("form").trigger("submit");
    await wrapper.get("form").trigger("submit");
    await wrapper.get('[aria-label="Close"]').trigger("click");
    expect(api.changePassword).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted("close")).toBeUndefined();
    expect(
      wrapper.findAll("input").every((input) => input.element.disabled),
    ).toBe(true);
    resolve();
    await flushPromises();
    wrapper.unmount();
  });
});
