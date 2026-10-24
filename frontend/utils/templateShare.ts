export function templateBrowseUrl(id: number): string {
  return new URL(`/templates/browse/${id}`, window.location.origin).href;
}

export async function copyTemplateBrowseUrl(id: number): Promise<string> {
  const url = templateBrowseUrl(id);
  await navigator.clipboard.writeText(url);
  return url;
}
