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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import type {
  ApiEndpoint,
  OpenApiParameter,
  OpenApiSchema,
} from '@/content/api-reference/types'
import {
  formatSchemaType,
  resolveSchema,
} from '@/content/api-reference/catalog'
import { cn } from '@/lib/utils'

function MethodBadge({ method, className }: { method: string; className?: string }) {
  const palette: Record<string, string> = {
    get: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30',
    post: 'bg-sky-500/10 text-sky-600 dark:text-sky-400 border-sky-500/30',
    put: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30',
    delete: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/30',
    patch: 'bg-violet-500/10 text-violet-600 dark:text-violet-400 border-violet-500/30',
    head: 'bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/30',
    options: 'bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/30',
  }
  return (
    <span
      className={cn(
        'inline-flex w-16 shrink-0 items-center justify-center rounded border px-1.5 py-0.5 text-xs font-bold tracking-wide uppercase',
        palette[method] ?? 'bg-slate-500/10 text-slate-600 dark:text-slate-400 border-slate-500/30',
        className
      )}
    >
      {method}
    </span>
  )
}

function ParamSchemaSummary({ schema }: { schema?: OpenApiSchema }) {
  if (!schema) return <span className='text-muted-foreground'>any</span>
  const type = formatSchemaType(schema)
  return <code className='text-[13px]'>{type}</code>
}

function isRequired(name: string, parent?: OpenApiSchema): boolean {
  return Boolean(parent?.required?.includes(name))
}

function SchemaFields({
  schema,
  doc,
  depth = 0,
}: {
  schema?: OpenApiSchema
  doc: ApiEndpoint['spec']
  depth?: number
}) {
  const resolved = resolveSchema(schema, doc)
  if (!resolved || (!resolved.properties && resolved.type !== 'object')) {
    return null
  }
  if (depth >= 4) {
    return <div className='text-muted-foreground text-xs'>…</div>
  }
  const entries = Object.entries(resolved.properties ?? {})
  return (
    <div className='flex flex-col'>
      {entries.map(([name, propSchema]) => {
        const nested = resolveSchema(propSchema, doc)
        const hasNested = Boolean(nested?.properties)
        return (
          <div
            key={name}
            className='flex flex-col gap-1 border-t py-2 first:border-t-0 first:pt-0'
            style={{ paddingLeft: depth * 12 }}
          >
            <div className='flex flex-wrap items-center gap-x-2 gap-y-0.5'>
              <code className='text-[13px] font-medium'>{name}</code>
              {isRequired(name, resolved) ? (
                <Badge variant='outline' className='h-4 px-1 text-[10px] text-rose-500'>
                  required
                </Badge>
              ) : null}
              <span className='text-xs text-muted-foreground'>
                <ParamSchemaSummary schema={propSchema} />
              </span>
              {nested?.example !== undefined ? (
                <span className='text-xs text-muted-foreground'>
                  example: <code>{String(nested.example)}</code>
                </span>
              ) : null}
            </div>
            {nested?.description ? (
              <p className='text-[13px] text-muted-foreground'>{nested.description}</p>
            ) : null}
            {hasNested ? <SchemaFields schema={nested} doc={doc} depth={depth + 1} /> : null}
          </div>
        )
      })}
    </div>
  )
}

function ParameterRow({ param }: { param: OpenApiParameter }) {
  return (
    <tr className='border-t'>
      <td className='py-2 pr-4 align-top'>
        <code className='text-[13px] font-medium'>{param.name}</code>
        {param.required ? (
          <span className='ml-1 text-xs text-rose-500'>*</span>
        ) : null}
      </td>
      <td className='py-2 pr-4 align-top text-xs text-muted-foreground'>{param.in}</td>
      <td className='py-2 pr-4 align-top'>
        <ParamSchemaSummary schema={param.schema} />
      </td>
      <td className='py-2 align-top text-[13px]'>{param.description ?? '—'}</td>
    </tr>
  )
}

export function EndpointDocumentation({ endpoint }: { endpoint: ApiEndpoint }) {
  const { t } = useTranslation()
  const { operation, spec, method, path } = endpoint
  const parameters = operation.parameters ?? []
  const bodySchema = operation.requestBody?.content?.['application/json']?.schema
  const responses = Object.entries(operation.responses ?? {}).sort(([a], [b]) =>
    Number(a) - Number(b)
  )
  const curlParts = [
    `curl -X ${method.toUpperCase()} "${path}"`,
    `  -H "Authorization: Bearer sk-xxxx"`,
  ]
  if (bodySchema) {
    curlParts.push(
      `  -H "Content-Type: application/json"`,
      `  -d '{}'`
    )
  }
  const curl = curlParts.join(' \\\n')

  return (
    <div className='flex flex-col gap-8'>
      {/* Method + path */}
      <div className='flex flex-col gap-3'>
        <div className='flex flex-wrap items-center gap-3'>
          <MethodBadge method={method} />
          <code className='rounded-md border bg-muted/60 px-2.5 py-1 text-sm break-all'>
            {path}
          </code>
        </div>
        <h1 className='scroll-m-20 text-2xl font-semibold tracking-tight lg:text-3xl'>
          {endpoint.summary}
        </h1>
        {endpoint.description ? (
          <div className='docs-description whitespace-pre-line text-[15px] leading-7 text-muted-foreground'>
            {endpoint.description}
          </div>
        ) : null}
        <div className='flex flex-wrap gap-2 text-xs text-muted-foreground'>
          {endpoint.deprecated ? <Badge variant='outline'>deprecated</Badge> : null}
          {endpoint.security.length ? (
            <span>
              {t('Authentication required')}: {endpoint.security.join(' / ')}
            </span>
          ) : (
            <span>{t('No authentication required')}</span>
          )}
        </div>
      </div>

      {/* Parameters */}
      {parameters.length ? (
        <section className='flex flex-col gap-3'>
          <h2 className='scroll-m-20 text-xl font-semibold tracking-tight'>
            {t('Parameters')}
          </h2>
          <div className='overflow-x-auto'>
            <table className='w-full text-sm'>
              <thead>
                <tr className='text-left text-xs text-muted-foreground uppercase'>
                  <th className='py-1 pr-4 font-medium'>Name</th>
                  <th className='py-1 pr-4 font-medium'>In</th>
                  <th className='py-1 pr-4 font-medium'>Type</th>
                  <th className='py-1 font-medium'>{t('Description')}</th>
                </tr>
              </thead>
              <tbody>
                {parameters.map((param) => (
                  <ParameterRow key={`${param.in}-${param.name}`} param={param} />
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}

      {/* Request body */}
      {bodySchema ? (
        <section className='flex flex-col gap-3'>
          <h2 className='scroll-m-20 text-xl font-semibold tracking-tight'>
            {t('Request Body')}
          </h2>
          <SchemaFields schema={bodySchema} doc={spec} />
        </section>
      ) : null}

      {/* Responses */}
      {responses.length ? (
        <section className='flex flex-col gap-3'>
          <h2 className='scroll-m-20 text-xl font-semibold tracking-tight'>
            {t('Response')}
          </h2>
          <div className='flex flex-col gap-2'>
            {responses.map(([code, response]) => (
              <div
                key={code}
                className='flex flex-col gap-1 rounded-lg border p-3 text-sm'
              >
                <div className='flex items-center gap-2'>
                  <Badge
                    variant='outline'
                    className={cn(
                      Number(code) >= 200 && Number(code) < 300
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-rose-600 dark:text-rose-400'
                    )}
                  >
                    {code}
                  </Badge>
                  <span className='text-muted-foreground'>{response.description}</span>
                </div>
              </div>
            ))}
          </div>
        </section>
      ) : null}

      {/* cURL example */}
      <section className='flex flex-col gap-3'>
        <h2 className='scroll-m-20 text-xl font-semibold tracking-tight'>cURL</h2>
        <pre className='overflow-x-auto rounded-lg border bg-muted/40 p-4 text-[13px] leading-6'><code>{curl}</code></pre>
      </section>
    </div>
  )
}
