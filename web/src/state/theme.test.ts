import { beforeEach, describe, expect, it } from "vitest";
import { useTheme } from "./theme";

const theme = useTheme();

beforeEach(() => {
  if (theme.isLight.value) theme.toggleTheme();
  window.localStorage.clear();
});

describe("theme state", () => {
  it("toggles the document palette and persists the choice", () => {
    theme.toggleTheme();

    expect(theme.isLight.value).toBe(true);
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(window.localStorage.getItem("ephemeral-theme")).toBe("light");

    theme.toggleTheme();

    expect(theme.isLight.value).toBe(false);
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(window.localStorage.getItem("ephemeral-theme")).toBe("dark");
  });
});
