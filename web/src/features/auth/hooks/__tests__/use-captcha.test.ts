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
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { useCaptcha } from '../use-captcha'

const { getCaptchaMock, useStatusMock } = vi.hoisted(() => ({
  getCaptchaMock: vi.fn(),
  useStatusMock: vi.fn(),
}))

vi.mock('@/features/auth/api', () => ({
  getCaptcha: getCaptchaMock,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: useStatusMock,
}))

vi.mock('sonner', () => ({
  toast: { info: vi.fn(), error: vi.fn(), success: vi.fn() },
}))

function useEnabledStatus(enabled: boolean) {
  useStatusMock.mockReturnValue({
    status: { captcha_enabled: enabled },
    loading: false,
    error: null,
  })
}

function challengeResponse(captchaId: string) {
  return {
    success: true,
    message: '',
    data: {
      captcha_id: captchaId,
      image: 'data:image/png;base64,iVBORw0KGgo=',
    },
  }
}

describe('image captcha challenge', () => {
  beforeEach(() => {
    getCaptchaMock.mockReset()
    useStatusMock.mockReset()
  })

  test('loads a challenge as soon as the captcha is enabled', async () => {
    useEnabledStatus(true)
    getCaptchaMock.mockResolvedValue(challengeResponse('challenge-1'))

    const { result } = renderHook(() => useCaptcha())

    await waitFor(() => expect(result.current.captchaId).toBe('challenge-1'))
    expect(result.current.captchaImage).toBe(
      'data:image/png;base64,iVBORw0KGgo='
    )
    expect(result.current.isCaptchaEnabled).toBe(true)
  })

  test('skips loading and validation when the captcha is disabled', async () => {
    useEnabledStatus(false)

    const { result } = renderHook(() => useCaptcha())

    await waitFor(() => expect(result.current.isCaptchaEnabled).toBe(false))
    expect(getCaptchaMock).not.toHaveBeenCalled()
    expect(result.current.validateCaptcha()).toBe(true)
  })

  test('blocks submission until a code is entered', async () => {
    useEnabledStatus(true)
    getCaptchaMock.mockResolvedValue(challengeResponse('challenge-2'))

    const { result } = renderHook(() => useCaptcha())
    await waitFor(() => expect(result.current.captchaId).toBe('challenge-2'))

    expect(result.current.validateCaptcha()).toBe(false)

    act(() => result.current.setCaptchaCode('A2B3'))

    expect(result.current.validateCaptcha()).toBe(true)
  })

  test('clears the entered code after refreshing the challenge', async () => {
    useEnabledStatus(true)
    getCaptchaMock.mockResolvedValue(challengeResponse('challenge-3'))

    const { result } = renderHook(() => useCaptcha())
    await waitFor(() => expect(result.current.captchaId).toBe('challenge-3'))

    act(() => result.current.setCaptchaCode('A2B3'))
    getCaptchaMock.mockResolvedValue(challengeResponse('challenge-4'))

    await act(async () => {
      await result.current.refreshCaptcha()
    })

    expect(result.current.captchaId).toBe('challenge-4')
    expect(result.current.captchaCode).toBe('')
  })
})
