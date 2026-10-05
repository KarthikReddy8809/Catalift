import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { ApiError, errorBody } from "@/lib/api";

import { catalogueKeys, uploadCsv, uploadImages, type ImagesResult, type Upload } from "../api";
import { UploadView, type UploadProblem, type UploadViewProps } from "../components/UploadView";
import { rememberLaunch } from "../launch";

function problemFor(err: unknown, fileName: string): UploadProblem {
  if (err instanceof ApiError && err.status === 413) return { kind: "too-large", fileName };
  if (err instanceof ApiError && err.status === 415) return { kind: "wrong-type", fileName };
  if (err instanceof ApiError && err.status === 409) return { kind: "conflict" };
  const id = errorBody(err)?.requestId ?? "no request id";
  return { kind: "server", requestId: id };
}

/** UploadPage: the CSV first, then the photos, against one upload (US-00-001). */
export function UploadPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [upload, setUpload] = useState<Upload | null>(null);
  const [images, setImages] = useState<ImagesResult | null>(null);
  const [problem, setProblem] = useState<UploadProblem | undefined>();
  const [file, setFile] = useState("");

  const csv = useMutation({
    mutationFn: uploadCsv,
    onMutate: (f) => {
      setFile(f.name);
      setProblem(undefined);
    },
    onSuccess: (u) => {
      setUpload(u);
      rememberLaunch({ uploadId: u.id, fileName: u.file_name });
      void queryClient.invalidateQueries({ queryKey: catalogueKeys.all });
    },
    onError: (err, f) => {
      setProblem(problemFor(err, f.name));
    },
  });
  const photos = useMutation({
    mutationFn: (files: File[]) => uploadImages(upload?.id ?? "", files),
    onMutate: () => {
      setProblem(undefined);
    },
    onSuccess: (r) => {
      setImages(r);
      void queryClient.invalidateQueries({ queryKey: catalogueKeys.all });
    },
    onError: (err, files) => {
      setProblem(problemFor(err, files[0]?.name ?? "photo"));
    },
  });

  const props: UploadViewProps = {
    step: csv.isPending ? "uploading" : images ? "images" : "csv",
    uploadingFile: file,
    progress: 50,
    onUploadCsv: (f) => {
      csv.mutate(f);
    },
    onUploadImages: (files) => {
      photos.mutate(files);
    },
    imagesUploading: photos.isPending,
    onDone: () => {
      void navigate({ to: "/products", search: upload ? { upload: upload.id } : {} });
    },
  };
  if (problem) props.problem = problem;
  if (upload) {
    props.summary = {
      fileName: upload.file_name,
      rowsTotal: upload.rows_total,
      rowsAccepted: upload.rows_accepted,
      rowsRejected: upload.rows_rejected,
      rowErrors: upload.row_errors.map((e) => ({
        rowNumber: e.row_number,
        sku: e.sku,
        reason: e.reason,
      })),
    };
  }
  if (images) {
    props.images = {
      attached: images.attached.length,
      unmatchedFiles: images.unmatched_files,
      productsMissingImage: images.products_missing_image,
      attachedFiles: images.attached.map((a) => ({ fileName: a.file_name, sku: a.sku })),
      rejectedFiles: images.rejected_files.map((f) => ({
        fileName: f.file_name,
        reason: f.reason,
      })),
    };
  }
  return (
    <div aria-busy={photos.isPending}>
      <UploadView {...props} />
    </div>
  );
}
