import { beforeEach, describe, expect, it } from 'vitest'
import { applyRouteSeo } from '@/utils/seo'

describe('applyRouteSeo', () => {
  beforeEach(() => {
    document.head.innerHTML = `
      <title>Sub2API - AI API Gateway</title>
      <meta name="description" content="old">
      <meta name="robots" content="index,follow">
      <link rel="canonical" href="/">
      <meta property="og:title" content="old">
      <meta property="og:description" content="old">
      <meta property="og:url" content="/">
      <meta property="og:image" content="/logo.png">
      <meta name="twitter:title" content="old">
      <meta name="twitter:description" content="old">
      <meta name="twitter:image" content="/logo.png">
    `
    window.history.replaceState({}, '', '/home?utm=1')
  })

  it('uses root canonical URL for the home route', () => {
    applyRouteSeo(
      {
        name: 'Home',
        path: '/',
        fullPath: '/',
        params: {},
        meta: {
          requiresAuth: false,
          title: 'Home',
          description: 'Unified AI access'
        }
      },
      {
        siteName: 'My Site',
        siteSubtitle: 'Unified AI access',
        siteLogo: '/brand.png'
      }
    )

    expect(document.title).toBe('Home - My Site')
    expect(document.querySelector('meta[name="description"]')?.getAttribute('content')).toBe('Unified AI access')
    expect(document.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('index,follow')
    expect(document.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('http://localhost:3000/')
    expect(document.querySelector('meta[property="og:image"]')?.getAttribute('content')).toBe('http://localhost:3000/brand.png')
  })

  it('marks authenticated routes as noindex', () => {
    applyRouteSeo({
      name: 'Dashboard',
      path: '/dashboard',
      fullPath: '/dashboard',
      params: {},
      meta: {
        requiresAuth: true,
        title: 'Dashboard'
      }
    })

    expect(document.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex,nofollow')
    expect(document.querySelector('link[rel="canonical"]')?.getAttribute('href')).toBe('http://localhost:3000/dashboard')
  })

  it('honors explicit noindex robots meta', () => {
    applyRouteSeo({
      name: 'Login',
      path: '/login',
      fullPath: '/login',
      params: {},
      meta: {
        requiresAuth: false,
        robots: 'noindex',
        title: 'Login'
      }
    })

    expect(document.querySelector('meta[name="robots"]')?.getAttribute('content')).toBe('noindex,nofollow')
  })
})
