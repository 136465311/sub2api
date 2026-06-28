import { i18n } from '@/i18n'
import { resolveRouteDocumentTitle } from '@/router/title'
import type { CustomMenuItem } from '@/types'
import type { RouteLocationNormalizedLoaded } from 'vue-router'

const DEFAULT_SITE_NAME = 'Sub2API'
const DEFAULT_DESCRIPTION =
  'Sub2API is an AI API gateway for Claude, GPT, Gemini and other models, with unified API keys, account routing, usage analytics and billing.'
const DEFAULT_KEYWORDS =
  'Sub2API, AI API Gateway, Claude API, GPT API, Gemini API, API Key, account pool, API billing'
const DEFAULT_IMAGE = '/logo.png'

type SeoRoute = Pick<
  RouteLocationNormalizedLoaded,
  'name' | 'path' | 'fullPath' | 'params' | 'meta'
>

interface ApplyRouteSeoOptions {
  siteName?: string
  siteSubtitle?: string
  siteLogo?: string
  customMenuItems?: CustomMenuItem[]
}

function normalizeContent(value: unknown): string {
  return typeof value === 'string' ? value.trim().replace(/\s+/g, ' ') : ''
}

function getAbsoluteUrl(pathOrUrl: string): string {
  if (typeof window === 'undefined') {
    return pathOrUrl
  }

  try {
    return new URL(pathOrUrl || '/', window.location.origin).toString()
  } catch {
    return window.location.origin
  }
}

function getCanonicalPath(route: SeoRoute): string {
  if (route.name === 'Home' || route.path === '/home') {
    return '/'
  }
  return route.path || '/'
}

function getRouteDescription(route: SeoRoute, siteSubtitle?: string): string {
  if (typeof route.meta.descriptionKey === 'string' && route.meta.descriptionKey.trim()) {
    const translated = i18n.global.t(route.meta.descriptionKey)
    if (translated && translated !== route.meta.descriptionKey) {
      return normalizeContent(translated)
    }
  }

  const subtitle = normalizeContent(siteSubtitle)
  if (route.name === 'Home' && subtitle) {
    return subtitle
  }

  if (typeof route.meta.description === 'string' && route.meta.description.trim()) {
    return normalizeContent(route.meta.description)
  }

  return subtitle || DEFAULT_DESCRIPTION
}

function findOrCreateMeta(selector: string, createAttrs: Record<string, string>): HTMLMetaElement {
  const existing = document.head.querySelector<HTMLMetaElement>(selector)
  if (existing) {
    return existing
  }

  const meta = document.createElement('meta')
  Object.entries(createAttrs).forEach(([key, value]) => meta.setAttribute(key, value))
  document.head.appendChild(meta)
  return meta
}

function setMetaName(name: string, content: string): void {
  const meta = findOrCreateMeta(`meta[name="${name}"]`, { name })
  meta.setAttribute('content', content)
}

function setMetaProperty(property: string, content: string): void {
  const meta = findOrCreateMeta(`meta[property="${property}"]`, { property })
  meta.setAttribute('content', content)
}

function setCanonical(href: string): void {
  let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'canonical'
    document.head.appendChild(link)
  }
  link.href = href
}

function getRobotsContent(route: SeoRoute): string {
  if (route.meta.robots === 'noindex') {
    return 'noindex,nofollow'
  }

  if (route.meta.requiresAuth !== false || route.name === 'NotFound') {
    return 'noindex,nofollow'
  }

  return 'index,follow'
}

export function applyRouteSeo(route: SeoRoute, options: ApplyRouteSeoOptions = {}): void {
  if (typeof document === 'undefined') {
    return
  }

  const customMenuItems = options.customMenuItems ?? []
  const siteName = normalizeContent(options.siteName) || DEFAULT_SITE_NAME
  const title = resolveRouteDocumentTitle(route, siteName, customMenuItems)
  const description = getRouteDescription(route, options.siteSubtitle)
  const canonical = getAbsoluteUrl(getCanonicalPath(route))
  const image = getAbsoluteUrl(normalizeContent(options.siteLogo) || DEFAULT_IMAGE)
  const robots = getRobotsContent(route)

  document.title = title
  setCanonical(canonical)

  setMetaName('description', description)
  setMetaName('keywords', DEFAULT_KEYWORDS)
  setMetaName('robots', robots)

  setMetaProperty('og:type', 'website')
  setMetaProperty('og:site_name', siteName)
  setMetaProperty('og:title', title)
  setMetaProperty('og:description', description)
  setMetaProperty('og:url', canonical)
  setMetaProperty('og:image', image)

  setMetaName('twitter:card', 'summary')
  setMetaName('twitter:title', title)
  setMetaName('twitter:description', description)
  setMetaName('twitter:image', image)
}
