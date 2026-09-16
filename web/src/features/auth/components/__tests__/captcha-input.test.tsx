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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, test, vi } from 'vitest'

import { CaptchaInput } from '../captcha-input'

const CAPTCHA_DATA_URL = 'data:image/png;base64,iVBORw0KGgo='
const CAPTCHA_LABEL = 'Graphical verification code'
const REFRESH_LABEL = 'Click to refresh'

function CaptchaHarness(props: {
  image?: string
  isLoading?: boolean
  onRefresh?: () => void
}) {
  const [code, setCode] = useState('')
  return (
    <CaptchaInput
      image={props.image ?? CAPTCHA_DATA_URL}
      code={code}
      onCodeChange={setCode}
      onRefresh={props.onRefresh ?? vi.fn()}
      isLoading={props.isLoading}
    />
  )
}

describe('captcha input', () => {
  test('uppercases typed characters and stops at the captcha code length', async () => {
    render(<CaptchaHarness />)

    await userEvent.type(screen.getByLabelText(CAPTCHA_LABEL), 'a2b3cd')

    expect(screen.getByLabelText(CAPTCHA_LABEL)).toHaveValue('A2B3')
  })

  test('requests a new challenge when the refresh control is clicked', async () => {
    const onRefresh = vi.fn()
    render(<CaptchaHarness onRefresh={onRefresh} />)

    await userEvent.click(screen.getByRole('button', { name: REFRESH_LABEL }))

    expect(onRefresh).toHaveBeenCalledTimes(1)
  })

  test('keeps the refresh control available when no image was loaded', async () => {
    const onRefresh = vi.fn()
    render(<CaptchaHarness image='' onRefresh={onRefresh} />)

    const refreshButton = screen.getByRole('button', { name: REFRESH_LABEL })
    expect(refreshButton.querySelector('img')).toBeNull()
    expect(refreshButton).toBeEnabled()

    await userEvent.click(refreshButton)

    expect(onRefresh).toHaveBeenCalledTimes(1)
  })
})
