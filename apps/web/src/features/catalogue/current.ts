import { queryOptions, useQuery } from "@tanstack/react-query";

import { apiFetch, ApiError } from "@/lib/api";

import { catalogueKeys, uploadSchema } from "./api";

/**
 * latestUploadQueryOptions reads the newest upload. Every screen shows only
 * its records; older uploads stay in the database. null before any upload.
 */
export function latestUploadQueryOptions() {
  return queryOptions({
    queryKey: catalogueKeys.latestUpload(),
    queryFn: async ({ signal }) => {
      try {
        return await apiFetch("/v1/uploads/latest", uploadSchema, { signal });
      } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
      }
    },
  });
}

export interface CurrentUpload {
  /** The newest upload's id; undefined before any upload or while loading. */
  id: string | undefined;
  fileName: string | undefined;
  /** False until the newest upload is known, so lists do not flash old rows. */
  ready: boolean;
}

/** useCurrentUpload is the upload every screen is scoped to: the newest one. */
export function useCurrentUpload(): CurrentUpload {
  const latest = useQuery(latestUploadQueryOptions());
  return {
    id: latest.data?.id,
    fileName: latest.data?.file_name,
    ready: latest.isSuccess || latest.isError,
  };
}
