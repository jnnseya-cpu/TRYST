// Profile and photo API client (backend/internal/profilesvc).
import { getToken } from "./api";
import { stripLocation } from "./stripExif";

export type PhotoView = {
  id: string;
  main: boolean;
  revealed: boolean;
  status: "live" | "checking";
  reason?: string;
  can_be_main: boolean;
  main_problem?: string;
  url: string;
};

export type OwnerProfile = {
  display_name: string;
  age: number | null;
  area: { id?: string; label?: string };
  city_only: boolean;
  photos: PhotoView[];
  max_photos: number;
  ready: boolean;
  problems: string[] | null;
};

export type Area = { id: string; district: string; city: string };

export type Card = {
  display_name: string;
  age: number | null;
  area: string;
  distance: string;
  verified: string;
  ready: boolean;
  photos: { url: string; main: boolean; blurred: boolean }[] | null;
};

/** Loads an image that needs the session token and returns an object URL for <img>. */
export async function imageURL(path: string): Promise<string | null> {
  const res = await fetch(path, { headers: { Authorization: `Bearer ${getToken()}` }, cache: "no-store" });
  if (!res.ok) return null;
  return URL.createObjectURL(await res.blob());
}

export type UploadResult = { ok: true; profile: OwnerProfile } | { ok: false; reason: string };

/** Strips location data on the device, then uploads one photo. */
export async function uploadPhoto(file: File): Promise<UploadResult> {
  if (!/^image\/(jpeg|png)$/.test(file.type)) return { ok: false, reason: "Please choose a JPEG or PNG photo." };
  if (file.size > 8 * 1024 * 1024) return { ok: false, reason: "Photos can be up to 8 MB." };
  const clean = stripLocation(new Uint8Array(await file.arrayBuffer()));
  const form = new FormData();
  form.append("photo", new Blob([clean as BlobPart], { type: file.type }), "photo");
  const res = await fetch("/v1/profile/photos", {
    method: "POST",
    headers: { Authorization: `Bearer ${getToken()}` },
    body: form,
    cache: "no-store",
  });
  const data = await res.json().catch(() => ({}));
  if (res.status === 201) return { ok: true, profile: data.profile as OwnerProfile };
  return { ok: false, reason: (data && data.title) || "That upload didn't work. Please try again." };
}
