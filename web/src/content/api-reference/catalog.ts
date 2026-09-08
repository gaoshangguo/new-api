/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'

import type {
  ApiCatalog,
  ApiCategory,
  ApiCategoryNode,
  ApiEndpoint,
  ApiGroup,
  ApiSpecSource,
  OpenApiDocument,
  OpenApiOperation,
  OpenApiSchema,
} from './types'

const AI_MODEL_SPEC_URL = '/openapi/relay.json'

const METHODS = ['get', 'post', 'put', 'delete', 'patch', 'head', 'options'] as const

type MethodKey = (typeof METHODS)[number]

function methodKeyOf(value: unknown): MethodKey | undefined {
  return METHODS.find((m) => typeof (value as Record<string, unknown>)?.[m] === 'object')
}

function pickOperation(
  pathItem: Record<string, unknown>
): { method: MethodKey; operation: OpenApiOperation } | undefined {
  const method = methodKeyOf(pathItem)
  if (!method) return undefined
  return { method, operation: pathItem[method] as OpenApiOperation }
}

function operationCategory(operation: OpenApiOperation): string {
  return operation.tags?.[0] ?? '其他'
}

function slugify(value: string): string {
  return (
    value
      .toLowerCase()
      .replaceAll(/[^\p{L}\p{N}]+/gu, '-')
      .replaceAll(/^-+|-+$/g, '')
      .slice(0, 64) || 'default'
  )
}

/** Human readable group metadata (official style). */
function groupMeta(source: ApiSpecSource): { id: ApiGroup['id']; title: string; description: string } {
  if (source === 'relay') {
    return {
      id: 'ai-model',
      title: 'AI 模型接口',
      description: 'AI 模型接口提供各种 AI 能力的调用，兼容 OpenAI API 格式。',
    }
  }
  return {
    id: 'management',
    title: '管理接口',
    description: '管理接口用于系统配置、用户管理、业务管理等后台操作。',
  }
}

function buildGroup(doc: OpenApiDocument, source: ApiSpecSource): ApiGroup {
  const meta = groupMeta(source)
  const categories = new Map<string, ApiEndpoint[]>()

  for (const [path, rawItem] of Object.entries(doc.paths ?? {})) {
    const picked = pickOperation(rawItem as Record<string, unknown>)
    if (!picked) continue
    const { method, operation } = picked
    const categoryId = operationCategory(operation)
    const summary =
      operation.summary ||
      (operation.description ?? '').split('\n')[0]?.trim() ||
      `${method.toUpperCase()} ${path}`

    const idParts = [source, slugify(categoryId), method, path]
      .filter(Boolean)
      .map((part) => slugify(String(part)))
    const id = idParts.join('--')

    const endpoint: ApiEndpoint = {
      id,
      groupId: meta.id,
      categoryId: slugify(categoryId),
      method,
      path,
      summary,
      description: operation.description ?? '',
      deprecated: operation.deprecated ?? false,
      operation,
      spec: doc,
      security: securityNames(operation, doc),
    }
    const list = categories.get(categoryId) ?? []
    list.push(endpoint)
    categories.set(categoryId, list)
  }

  const categoryList: ApiCategory[] = [...categories.entries()]
    .map(([title, endpoints]) => ({
      id: slugify(title),
      title,
      endpoints: endpoints.sort((a, b) => a.path.localeCompare(b.path)),
    }))
    .sort((a, b) => a.title.localeCompare(b.title, 'zh-CN'))

  return { ...meta, categories: categoryList }
}

function securityNames(
  operation: OpenApiOperation,
  doc: OpenApiDocument
): string[] {
  const requirements = operation.security ?? doc.security
  const names = new Set<string>()
  for (const req of requirements ?? []) {
    for (const key of Object.keys(req)) {
      names.add(key)
    }
  }
  return [...names]
}

async function fetchSpec(url: string): Promise<OpenApiDocument> {
  const res = await fetch(url)
  if (!res.ok) {
    throw new Error(`加载 OpenAPI 失败：${url} (${res.status})`)
  }
  return (await res.json()) as OpenApiDocument
}

async function loadAiModel(): Promise<ApiGroup> {
  return buildGroup(await fetchSpec(AI_MODEL_SPEC_URL), 'relay')
}

export async function loadApiCatalog(): Promise<ApiCatalog> {
  const aiModel = await loadAiModel()
  const groups = [aiModel]
  const endpointCount = groups.reduce(
    (sum, group) =>
      sum + group.categories.reduce((s, c) => s + c.endpoints.length, 0),
    0
  )
  return { groups, endpointCount }
}

export function useApiCatalog() {
  return useQuery({
    queryKey: ['docs-api-reference-catalog'],
    queryFn: loadApiCatalog,
    staleTime: 15 * 60 * 1000,
  })
}

export function findEndpoint(
  catalog: ApiCatalog,
  id: string
): ApiEndpoint | undefined {
  for (const group of catalog.groups) {
    for (const category of group.categories) {
      const hit = category.endpoints.find((e) => e.id === id)
      if (hit) return hit
    }
  }
  return undefined
}

/** Resolve a $ref against the document schemas (with cycle protection). */
export function resolveSchema(
  schema: OpenApiSchema | undefined,
  doc: OpenApiDocument,
  seen = new Set<string>()
): OpenApiSchema | undefined {
  if (!schema) return undefined
  if (!schema.$ref) return schema
  const name = schema.$ref.split('/').pop()
  if (!name || seen.has(name)) return { type: 'object', description: name }
  const next = new Set(seen)
  next.add(name)
  const ref = doc.components?.schemas?.[name]
  return ref ? resolveSchema(ref, doc, next) : { type: 'object', description: name }
}

export function formatSchemaType(schema: OpenApiSchema | undefined): string {
  if (!schema) return 'any'
  if (schema.$ref) {
    return schema.$ref.split('/').pop() ?? 'object'
  }
  if (schema.type === 'array') {
    return `array<${formatSchemaType(schema.items)}>`
  }
  if (schema.type === 'object' || schema.properties) return 'object'
  return schema.type ?? (schema.enum ? 'enum' : 'any')
}

/**
 * Group flat categories into a hierarchy by splitting titles on "/".
 * e.g. "视频生成/即梦格式", "视频生成/Kling格式" nest under a "视频生成" node.
 * A parent node may also carry its own endpoints (tag exactly "视频生成").
 */
export function buildCategoryTree(categories: ApiCategory[]): ApiCategoryNode[] {
  const index = new Map<string, ApiCategoryNode>()
  const roots: ApiCategoryNode[] = []

  const ensureNode = (path: string): ApiCategoryNode => {
    let node = index.get(path)
    if (node) return node
    const title = path.includes('/') ? path.slice(path.lastIndexOf('/') + 1) : path
    node = { path, title, endpoints: [], children: [] }
    index.set(path, node)
    const slash = path.lastIndexOf('/')
    if (slash === -1) {
      roots.push(node)
    } else {
      const parent = ensureNode(path.slice(0, slash))
      parent.children.push(node)
    }
    return node
  }

  for (const category of categories) {
    const segments = category.title
      .split('/')
      .map((s) => s.trim())
      .filter(Boolean)
    if (segments.length === 0) continue
    const node = ensureNode(segments.join('/'))
    node.endpoints.push(...category.endpoints)
  }

  const sortRecursive = (nodes: ApiCategoryNode[]) => {
    nodes.sort((a, b) => a.title.localeCompare(b.title, 'zh-CN'))
    for (const node of nodes) sortRecursive(node.children)
  }
  sortRecursive(roots)

  return roots
}

export { slugify }
