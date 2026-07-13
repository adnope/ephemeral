import type { Item } from "../types";

export async function copyImageToClipboard(item: Item) {
  if (!navigator.clipboard?.write || typeof ClipboardItem === "undefined") {
    throw new Error("Image copying is not supported by this browser");
  }

  const response = await fetch(item.contentUrl);
  if (!response.ok) throw new Error("Could not load the image");
  const source = await response.blob();
  let png = source;
  if (source.type !== "image/png") {
    const bitmap = await createImageBitmap(source);
    const canvas = document.createElement("canvas");
    canvas.width = bitmap.width;
    canvas.height = bitmap.height;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("Could not prepare the image for copying");
    context.drawImage(bitmap, 0, 0);
    bitmap.close();
    png = await new Promise<Blob>((resolve, reject) =>
      canvas.toBlob(
        (blob) =>
          blob ? resolve(blob) : reject(new Error("Image conversion failed")),
        "image/png",
      ),
    );
  }
  await navigator.clipboard.write([new ClipboardItem({ "image/png": png })]);
}
