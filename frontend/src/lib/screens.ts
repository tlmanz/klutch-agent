export type ScreenId = 'printers' | 'jobs' | 'updates' | 'settings'

export const SCREEN_TITLES: Record<ScreenId, string> = {
  printers: 'Printers',
  jobs: 'Jobs',
  updates: 'Updates',
  settings: 'Settings',
}

export const SCREEN_ORDER: ScreenId[] = ['printers', 'jobs', 'updates', 'settings']
