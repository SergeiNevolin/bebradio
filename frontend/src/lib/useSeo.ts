import { useEffect } from 'react'

interface SeoOptions {
  /** Полный <title> страницы. */
  title: string
  /** meta[name="description"] и og:description. */
  description?: string
  /** Абсолютный canonical URL и og:url. */
  canonical?: string
}

/**
 * Проставляет метаданные текущей страницы SPA: <title>, description, canonical
 * и соответствующие og-теги, возвращая их к прежним значениям при уходе.
 * Роботы, исполняющие JS (Google, Яндекс), видят эти значения; для надёжной
 * индексации публичные маршруты дополнительно пререндерятся.
 */
export function useSeo({ title, description, canonical }: SeoOptions) {
  useEffect(() => {
    const metaDescription = document.querySelector<HTMLMetaElement>('meta[name="description"]')
    const linkCanonical = document.querySelector<HTMLLinkElement>('link[rel="canonical"]')
    const ogTitle = document.querySelector<HTMLMetaElement>('meta[property="og:title"]')
    const ogDescription = document.querySelector<HTMLMetaElement>('meta[property="og:description"]')
    const ogUrl = document.querySelector<HTMLMetaElement>('meta[property="og:url"]')

    const prev = {
      title: document.title,
      description: metaDescription?.content,
      canonical: linkCanonical?.href,
      ogTitle: ogTitle?.content,
      ogDescription: ogDescription?.content,
      ogUrl: ogUrl?.content,
    }

    document.title = title
    if (metaDescription && description) metaDescription.content = description
    if (linkCanonical && canonical) linkCanonical.href = canonical
    if (ogTitle) ogTitle.content = title
    if (ogDescription && description) ogDescription.content = description
    if (ogUrl && canonical) ogUrl.content = canonical

    return () => {
      document.title = prev.title
      if (metaDescription && prev.description !== undefined) metaDescription.content = prev.description
      if (linkCanonical && prev.canonical !== undefined) linkCanonical.href = prev.canonical
      if (ogTitle && prev.ogTitle !== undefined) ogTitle.content = prev.ogTitle
      if (ogDescription && prev.ogDescription !== undefined) ogDescription.content = prev.ogDescription
      if (ogUrl && prev.ogUrl !== undefined) ogUrl.content = prev.ogUrl
    }
  }, [title, description, canonical])
}
