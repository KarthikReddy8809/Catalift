/** uploadQuery adds the upload filter to a list path; none means every upload. */
export function uploadQuery(path: string, uploadId: string | undefined): string {
  if (!uploadId) return path;
  return `${path}${path.includes("?") ? "&" : "?"}filter[upload_id]=${encodeURIComponent(uploadId)}`;
}
