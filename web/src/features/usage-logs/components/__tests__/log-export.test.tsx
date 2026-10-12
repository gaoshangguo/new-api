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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { AxiosError, AxiosHeaders, type AxiosResponse } from 'axios'
import i18next from 'i18next'
import { Toaster, toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import type { GetLogsParams } from '../../types'
import { CommonLogsExportButton } from '../common-logs-export-button'
import { LogsFilterToolbar } from '../logs-filter-toolbar'

const mime = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

function Fixture(props: {
  params?: GetLogsParams
  admin?: boolean
  toolbar?: boolean
}) {
  const table = useReactTable({
    data: [],
    columns: [],
    getCoreRowModel: getCoreRowModel(),
  })
  const button = (
    <CommonLogsExportButton
      params={props.params ?? {}}
      isAdmin={props.admin ?? false}
    />
  )
  if (!props.toolbar) return button
  return (
    <LogsFilterToolbar
      table={table}
      primaryFilters={<input aria-label='Model' />}
      mobilePinnedFilters={<input aria-label='Time range' />}
      hasActiveFilters={false}
      onReset={() => {}}
      onSearch={() => {}}
      actionEnd={button}
    />
  )
}

function renderExport(props: Parameters<typeof Fixture>[0] = {}) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <Fixture {...props} />
      <Toaster />
    </QueryClientProvider>
  )
}

function response(data: Blob): AxiosResponse<Blob> {
  return {
    data,
    status: 200,
    statusText: 'OK',
    headers: new AxiosHeaders(),
    config: { headers: new AxiosHeaders() },
  }
}

describe('usage log Excel export', () => {
  beforeEach(async () => {
    await i18next.changeLanguage('en')
    class DownloadURL extends URL {
      static createObjectURL = vi.fn(() => 'blob:usage-logs')
      static revokeObjectURL = vi.fn()
    }
    vi.stubGlobal('URL', DownloadURL)
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
  })
  afterEach(() => {
    toast.dismiss()
    vi.unstubAllGlobals()
  })

  test.each([false, true])(
    'places Export immediately after View on mobile=%s',
    (mobile) => {
      const matchMedia = window.matchMedia
      vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
        ...matchMedia(query),
        matches: mobile,
      }))
      renderExport({ toolbar: true })
      const buttons = screen.getAllByRole('button')
      const viewIndex = buttons.indexOf(
        screen.getByRole('button', { name: 'View' })
      )
      expect(buttons[viewIndex + 1]).toBe(
        screen.getByRole('button', { name: 'Export' })
      )
      if (mobile) expect(screen.getByLabelText('Time range')).toBeVisible()
    }
  )

  test('downloads every matching admin log with all filters and no page parameters', async () => {
    const blob = new Blob(['xlsx'], { type: mime })
    const get = vi.spyOn(api, 'get').mockResolvedValue(response(blob))
    renderExport({
      admin: true,
      params: {
        p: 3,
        page_size: 20,
        type: 2,
        start_timestamp: 100,
        end_timestamp: 200,
        model_name: 'gpt_%',
        username: 'alice',
        token_name: 'personal',
        channel: 1,
        group: 'default',
        request_id: 'request',
        upstream_request_id: 'upstream',
      },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    await waitFor(() => expect(URL.createObjectURL).toHaveBeenCalledWith(blob))
    const url = new URL(String(get.mock.calls[0][0]), 'https://example.test')
    expect(url.pathname).toBe('/api/log/export')
    expect(Object.fromEntries(url.searchParams)).toEqual({
      type: '2',
      start_timestamp: '100',
      end_timestamp: '200',
      model_name: 'gpt_%',
      username: 'alice',
      token_name: 'personal',
      channel: '1',
      group: 'default',
      request_id: 'request',
      upstream_request_id: 'upstream',
    })
    const click = vi.mocked(HTMLAnchorElement.prototype.click)
    const anchor = click.mock.instances[0] as HTMLAnchorElement
    expect(anchor.download).toBe('usage-logs.xlsx')
    expect(anchor.href).toBe('blob:usage-logs')
    expect(anchor.isConnected).toBe(false)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:usage-logs')
  })

  test('uses the personal endpoint and excludes admin-only filters in self view', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue(response(new Blob(['xlsx'], { type: mime })))
    renderExport({
      params: { username: 'other', channel: 7, model_name: 'gpt', type: 0 },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    await waitFor(() => expect(URL.createObjectURL).toHaveBeenCalled())
    const url = new URL(String(get.mock.calls[0][0]), 'https://example.test')
    expect(url.pathname).toBe('/api/log/self/export')
    expect(Object.fromEntries(url.searchParams)).toEqual({
      model_name: 'gpt',
      type: '0',
    })
  })

  test('disables the export button and announces busy while the download request is pending', async () => {
    let resolve: (value: AxiosResponse<Blob>) => void = () => {
      throw new Error('missing resolver')
    }
    vi.spyOn(api, 'get').mockImplementation(
      () =>
        new Promise((done) => {
          resolve = done
        })
    )
    renderExport()
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    const pending = await screen.findByRole('button', { name: 'Processing...' })
    expect(pending).toBeDisabled()
    expect(pending).toHaveAttribute('aria-busy', 'true')
    resolve(response(new Blob(['xlsx'], { type: mime })))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Export' })).toBeEnabled()
    )
  })

  test('shows the row-limit error without downloading JSON as an Excel file', async () => {
    const data = new Blob([], { type: 'application/json' })
    Object.defineProperty(data, 'text', {
      value: async () =>
        JSON.stringify({ success: false, code: 'LOG_EXPORT_LIMIT' }),
    })
    const errorResponse = { ...response(data), status: 400 }
    vi.spyOn(api, 'get').mockRejectedValue(
      new AxiosError(
        'Bad Request',
        'ERR_BAD_REQUEST',
        errorResponse.config,
        undefined,
        errorResponse
      )
    )
    renderExport()
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    expect(
      await screen.findByText(
        'Export is limited to 10,000 logs. Please narrow the filters.'
      )
    ).toBeVisible()
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Export' })).toBeEnabled()
  })

  test('rejects an unexpected proxy response without creating a broken download', async () => {
    vi.spyOn(api, 'get').mockResolvedValue(
      response(new Blob(['<html>Error</html>'], { type: 'text/html' }))
    )
    renderExport()
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    expect(
      await screen.findByText(
        'Log export failed. Please narrow the filters or try again.'
      )
    ).toBeVisible()
    expect(URL.createObjectURL).not.toHaveBeenCalled()
  })
})
