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
import { describe, expect, test } from 'vitest'

import { buildSample, type SampleContext } from '../../lib/api-samples'

const videoCtx: SampleContext = {
  baseUrl: 'https://api.example.com',
  apiKeyEnv: 'NEW_API_KEY',
  modelName: 'seedance-video',
  endpointType: 'openai-video',
  endpointPath: '/v1/videos',
}

describe('buildSample - video endpoint', () => {
  test('curl sample creates a task then queries it, not a chat request', () => {
    const code = buildSample('curl', 'openai-video', videoCtx)

    expect(code).toContain('Create a video generation task')
    expect(code).toContain('/v1/videos')
    expect(code).toContain('/v1/videos/{video_id}')
    expect(code).toContain('"model": "seedance-video"')
    expect(code).not.toContain('/v1/chat/completions')
    expect(code).not.toContain('messages')
  })

  test('python sample posts the task then reads it back', () => {
    const code = buildSample('python', 'openai-video', videoCtx)

    expect(code).toContain('requests.post')
    expect(code).toContain('requests.get')
    expect(code).toContain('task["id"]')
    expect(code).toContain('/v1/videos')
  })

  test('chat endpoint still renders the OpenAI chat sample', () => {
    const code = buildSample('curl', 'openai', {
      ...videoCtx,
      endpointType: 'openai',
      endpointPath: '/v1/chat/completions',
    })

    expect(code).toContain('/v1/chat/completions')
    expect(code).toContain('messages')
    expect(code).not.toContain('Create a video generation task')
  })
})