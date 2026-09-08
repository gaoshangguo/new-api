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

/** Minimal OpenAPI subset used to build the docs-style API reference. */

export type HttpMethod =
  | 'get'
  | 'post'
  | 'put'
  | 'delete'
  | 'patch'
  | 'head'
  | 'options'

export interface OpenApiSchema {
  type?: string
  format?: string
  description?: string
  example?: unknown
  default?: unknown
  required?: string[]
  properties?: Record<string, OpenApiSchema>
  items?: OpenApiSchema
  enum?: unknown[]
  $ref?: string
  nullable?: boolean
}

export interface OpenApiParameter {
  name: string
  in: 'path' | 'query' | 'header' | 'cookie'
  description?: string
  required?: boolean
  deprecated?: boolean
  schema?: OpenApiSchema
  example?: unknown
}

export interface OpenApiOperation {
  summary?: string
  description?: string
  deprecated?: boolean
  operationId?: string
  tags?: string[]
  parameters?: OpenApiParameter[]
  requestBody?: {
    required?: boolean
    description?: string
    content?: Record<string, { schema?: OpenApiSchema }>
  }
  responses?: Record<
    string,
    { description?: string; content?: Record<string, { schema?: OpenApiSchema }> }
  >
  security?: Array<Record<string, string[]>>
}

export interface OpenApiPathItem {
  get?: OpenApiOperation
  put?: OpenApiOperation
  post?: OpenApiOperation
  delete?: OpenApiOperation
  patch?: OpenApiOperation
  head?: OpenApiOperation
  options?: OpenApiOperation
}

export interface OpenApiDocument {
  openapi?: string
  info?: { title?: string; description?: string; version?: string }
  servers?: Array<{ url?: string }>
  tags?: Array<{ name: string; description?: string }>
  paths: Record<string, OpenApiPathItem>
  components?: {
    schemas?: Record<string, OpenApiSchema>
    securitySchemes?: Record<string, { type?: string; name?: string }>
  }
  security?: Array<Record<string, string[]>>
}

/** Normalized endpoint that powers the reference UI. */
export interface ApiEndpoint {
  id: string
  groupId: string
  categoryId: string
  method: HttpMethod
  path: string
  summary: string
  description: string
  deprecated: boolean
  operation: OpenApiOperation
  spec: OpenApiDocument
  security: string[]
}

export interface ApiCategory {
  id: string
  title: string
  description?: string
  endpoints: ApiEndpoint[]
}

export interface ApiGroup {
  id: 'ai-model' | 'management'
  title: string
  description?: string
  categories: ApiCategory[]
}

export interface ApiCatalog {
  groups: ApiGroup[]
  endpointCount: number
}

export type ApiSpecSource = 'relay' | 'management'
