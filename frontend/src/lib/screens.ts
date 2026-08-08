export type ScreenId = 'printers' | 'print' | 'jobs' | 'updates' | 'settings'

export const SCREEN_TITLES: Record<ScreenId, string> = {
  printers: 'Printers',
  print: 'Print a file',
  jobs: 'Jobs',
  updates: 'Updates',
  settings: 'Settings',
}

export const SCREEN_ORDER: ScreenId[] = ['printers', 'print', 'jobs', 'updates', 'settings']
