const ID_RE = /(?:v=|youtu\.be\/|\/shorts\/|\/embed\/|\/live\/)([\w-]{11})/

/** Extract the 11-character YouTube video id from a watch / short / embed URL. */
export function youtubeId(url: string | null | undefined): string {
  if (!url) return ''
  const m = ID_RE.exec(url)
  return m ? m[1] : ''
}
