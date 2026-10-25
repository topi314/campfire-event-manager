export type ImportedPack = {
  id: number;
  name: string;
};

export type PackImportResult = {
  packs?: ImportedPack[];
  created?: number;
};

async function errorMessage(res: Response, fallback: string) {
  try {
    const body = await res.json();
    if (body?.error) return String(body.error);
  } catch {
    /* ignore */
  }
  return fallback;
}

export async function downloadPacks(base: string, ids: number[], filename: string) {
  const res = await fetch(`${base}/api/icon-libraries/export`, {
    method: "POST",
    credentials: "include",
    headers: {
      Accept: "application/zip",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ ids }),
  });
  if (!res.ok) {
    throw new Error(await errorMessage(res, "Failed to export packs"));
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export function packDownloadName(name: string, count: number) {
  if (count !== 1) return "campfire-packs.zip";
  const safe = name.replace(/[^\w.-]+/g, "_").replace(/^_+|_+$/g, "");
  return `${safe || "pack"}.zip`;
}

export async function importPackFiles(base: string, files: File[]): Promise<PackImportResult> {
  const body = new FormData();
  for (const file of files) body.append("file", file);
  const res = await fetch(`${base}/api/icon-libraries/import`, {
    method: "POST",
    credentials: "include",
    body,
  });
  if (!res.ok) {
    throw new Error(await errorMessage(res, "Import failed"));
  }
  return (await res.json()) as PackImportResult;
}
