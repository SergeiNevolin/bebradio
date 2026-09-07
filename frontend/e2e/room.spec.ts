import { expect, test } from '@playwright/test'

test('creates a room and opens it', async ({ page }) => {
  await page.route('**/api/rooms', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: [] })
      return
    }

    expect(route.request().method()).toBe('POST')
    expect(route.request().postDataJSON()).toEqual({
      name: 'Friday party',
      password: 'secret123',
    })

    await route.fulfill({
      status: 201,
      json: {
        id: 'ABC123',
        name: 'Friday party',
        access: 'room-access-token',
      },
    })
  })

  await page.goto('/')
  await page.getByRole('button', { name: 'Create Room' }).click()

  await expect(page.getByRole('heading', { name: 'Create a room' })).toBeVisible()
  await page.getByPlaceholder('Room name').fill('Friday party')
  await page.getByPlaceholder('Leave empty for an open room').fill('secret123')
  await page.getByRole('button', { name: 'Create room', exact: true }).click()

  await expect(page).toHaveURL(/\/room\/ABC123$/)
})

test('adds a YouTube track to the room queue', async ({ page }) => {
  await page.addInitScript(() => {
    class TestWebSocket {
      static OPEN = 1
      readyState = TestWebSocket.OPEN
      onopen: ((event: Event) => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: ((event: CloseEvent) => void) | null = null
      onerror: ((event: Event) => void) | null = null

      constructor() {
        setTimeout(() => this.onopen?.(new Event('open')), 0)
      }

      send() {}
      close() {}
    }

    window.WebSocket = TestWebSocket as unknown as typeof window.WebSocket
  })

  await page.route('**/api/rooms/ROOM42', async (route) => {
    await route.fulfill({
      json: {
        id: 'ROOM42',
        name: 'Test room',
        owner_id: 'owner-1',
        queue: [],
        current_index: 0,
        is_playing: false,
        position: 0,
        current_track: null,
        user_count: 0,
        listeners: [],
        allow_anonymous_add: true,
        is_private: false,
        has_password: false,
        track_votes: { likes: 0, dislikes: 0 },
        skip_voters: [],
      },
    })
  })

  await page.route('**/api/rooms/ROOM42/queue*', async (route) => {
    expect(route.request().method()).toBe('POST')
    expect(route.request().postDataJSON()).toEqual({
      url: 'https://www.youtube.com/watch?v=track42',
    })
    await route.fulfill({ status: 201, json: { id: 'track-42' } })
  })

  await page.goto('/room/ROOM42')
  const trackInput = page.getByPlaceholder('Search or paste YouTube URL...')
  await trackInput.fill('https://www.youtube.com/watch?v=track42')
  await page.getByRole('button', { name: 'Add', exact: true }).click()

  await expect(page.getByText('Added!', { exact: true })).toBeVisible()
})

test('shows the current track while the room is playing', async ({ page }) => {
  const track = {
    id: 'track-42',
    title: 'Midnight Drive',
    artist: 'Test Artist',
    url: 'https://cdn.example.test/track-42.m4a',
    thumbnail: '',
    duration: 210,
    added_by: 'tester',
  }

  await page.addInitScript(() => {
    class TestWebSocket {
      static OPEN = 1
      readyState = TestWebSocket.OPEN
      onopen: ((event: Event) => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onclose: ((event: CloseEvent) => void) | null = null
      onerror: ((event: Event) => void) | null = null

      constructor() {
        setTimeout(() => this.onopen?.(new Event('open')), 0)
      }

      send() {}
      close() {}
    }

    window.WebSocket = TestWebSocket as unknown as typeof window.WebSocket
  })

  await page.addInitScript(() => {
    HTMLMediaElement.prototype.play = function () {
      this.dataset.playRequested = 'true'
      return Promise.resolve()
    }
  })

  await page.route('**/api/rooms/PLAY42', async (route) => {
    await route.fulfill({
      json: {
        id: 'PLAY42',
        name: 'Playing room',
        owner_id: 'owner-1',
        queue: [track],
        current_index: 0,
        is_playing: true,
        position: 24,
        current_track: track,
        user_count: 1,
        listeners: [],
        allow_anonymous_add: true,
        is_private: false,
        has_password: false,
        track_votes: { likes: 0, dislikes: 0 },
        skip_voters: [],
      },
    })
  })

  await page.goto('/room/PLAY42')

  await expect(page.getByText('Midnight Drive', { exact: true })).toHaveCount(2)
  await expect(page.getByText('Test Artist', { exact: true })).toHaveCount(2)
  await expect(page.locator('audio')).toHaveCount(2)
  await expect(page.locator('audio[data-play-requested="true"]')).toHaveCount(1)
})