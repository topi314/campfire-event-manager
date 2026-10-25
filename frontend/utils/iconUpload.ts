export const iconUploadBatchSize = 40;

export type IconUploadFile = {
  file: File;
  name: string;
  category: string;
  pokemon: string[];
};

export async function uploadIconsInBatches(
  post: (body: FormData) => Promise<{ errors?: string[] }>,
  files: IconUploadFile[],
  onProgress?: (done: number, total: number) => void,
): Promise<{ errors: string[]; uploaded: number }> {
  const total = files.length;
  let uploaded = 0;
  const errors: string[] = [];
  for (let i = 0; i < files.length; i += iconUploadBatchSize) {
    const slice = files.slice(i, i + iconUploadBatchSize);
    const body = new FormData();
    body.append(
      "meta",
      JSON.stringify(
        slice.map((item) => ({
          name: item.name.trim(),
          category: item.category,
          pokemon: item.pokemon,
        })),
      ),
    );
    for (const item of slice) body.append("file", item.file);
    try {
      const res = await post(body);
      if (res.errors?.length) errors.push(...res.errors);
    } catch (e) {
      errors.push(e instanceof Error ? e.message : "Upload failed");
      return { errors, uploaded };
    }
    uploaded += slice.length;
    onProgress?.(uploaded, total);
  }
  return { errors, uploaded };
}
