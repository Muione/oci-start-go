import client from './client'
import type { Bucket, StorageObject, MultipartUpload } from '@/types/api'

// ─── Namespace ─────────────────────────────────────────────────────────

export function getNamespace(tenantId: number): Promise<{ namespace: string }> {
  return client.get<unknown, { namespace: string }>('/oci/storage/namespace', { params: { tenantId } })
}

// ─── Buckets ───────────────────────────────────────────────────────────

export interface BucketListResponse {
  items: Bucket[]
  nextPage?: string
}

export function listBuckets(tenantId: number, limit = 100, pageToken?: string): Promise<BucketListResponse> {
  return client.get<unknown, BucketListResponse>('/oci/storage/buckets', {
    params: { tenantId, limit, ...(pageToken ? { pageToken } : {}) },
  })
}

export function createBucket(tenantId: number, bucketName: string, publicAccessType = 'NoPublicAccess'): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/storage/bucket/create', { tenantId, bucketName, publicAccessType })
}

export function deleteBucket(tenantId: number, namespace: string, bucketName: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/storage/bucket/delete', { tenantId, namespace, bucketName })
}

// ─── Objects ───────────────────────────────────────────────────────────

export interface ObjectListResponse {
  items: StorageObject[]
  nextStartWith?: string
}

export function listObjects(
  tenantId: number, namespace: string, bucketName: string,
  limit = 100, prefix?: string, startToken?: string,
): Promise<ObjectListResponse> {
  return client.get<unknown, ObjectListResponse>('/oci/storage/objects', {
    params: { tenantId, namespace, bucketName, limit, ...(prefix ? { prefix } : {}), ...(startToken ? { startToken } : {}) },
  })
}

export function deleteObject(tenantId: number, namespace: string, bucketName: string, objectName: string): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/storage/object/delete', { tenantId, namespace, bucketName, objectName })
}

export function generatePresignedUrl(
  tenantId: number, namespace: string, bucketName: string, objectName: string, validitySeconds = 3600,
): Promise<{ url: string }> {
  return client.post<unknown, { url: string }>('/oci/storage/object/presigned', {
    tenantId, namespace, bucketName, objectName, validitySeconds,
  })
}

// ─── Download / Preview URLs ───────────────────────────────────────────

export function downloadObjectUrl(tenantId: number, namespace: string, bucketName: string, objectName: string): string {
  const p = new URLSearchParams({ tenantId: String(tenantId), namespace, bucketName, objectName })
  return `/oci/storage/object/download?${p.toString()}`
}

export function previewObjectUrl(tenantId: number, namespace: string, bucketName: string, objectName: string): string {
  const p = new URLSearchParams({ tenantId: String(tenantId), namespace, bucketName, objectName })
  return `/oci/storage/object/preview?${p.toString()}`
}

// ─── Upload (single PUT) ───────────────────────────────────────────────

export function uploadObjectUrl(): string {
  return '/oci/storage/object/upload'
}

// ─── Multipart Upload ──────────────────────────────────────────────────

export interface MultipartInitResponse {
  uploadId: string
}

export function multipartInitiate(
  tenantId: number, namespace: string, bucketName: string,
  objectName: string, contentType: string, totalSize: number, chunkSize: number,
): Promise<MultipartInitResponse> {
  return client.post<unknown, MultipartInitResponse>('/oci/storage/object/multipart/initiate', {
    tenantId, namespace, bucketName, objectName, contentType, totalSize, chunkSize,
  })
}

export interface PartResult {
  etag: string
}

export function multipartUploadPartUrl(): string {
  return '/oci/storage/object/multipart/part'
}

export interface CommitPart {
  partNum: number
  etag: string
}

export function multipartCommit(
  tenantId: number, namespace: string, bucketName: string,
  objectName: string, uploadId: string, parts: CommitPart[],
): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/storage/object/multipart/commit', {
    tenantId, namespace, bucketName, objectName, uploadId, parts,
  })
}

export function multipartAbort(
  tenantId: number, namespace: string, bucketName: string,
  objectName: string, uploadId: string,
): Promise<unknown> {
  return client.post<unknown, unknown>('/oci/storage/object/multipart/abort', {
    tenantId, namespace, bucketName, objectName, uploadId,
  })
}

export function listResumableUploads(tenantId: number, bucketName: string): Promise<MultipartUpload[]> {
  return client.get<unknown, MultipartUpload[]>('/oci/storage/object/multipart/resumeable', {
    params: { tenantId, bucketName },
  })
}
