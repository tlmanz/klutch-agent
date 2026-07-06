// The Klutch brand badge: the two-tone K glyph (amber "wing" + light body,
// matching the ui.pen LogoBadge) inside an amber-outlined tile. Inlined so both
// fills render correctly regardless of theme.
export function Logo({ size = 36 }: { size?: number }) {
  const glyph = size * 0.52
  return (
    <div
      style={{ width: size, height: size }}
      className="flex shrink-0 items-center justify-center rounded-[10px] border-[1.5px] border-amber bg-base"
    >
      <svg
        width={glyph}
        height={glyph}
        viewBox="0 0 131.84 153.61"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="m52.47,47.94L117.07.1c.09-.06.19-.1.3-.1h13.98c.44,0,.66.52.37.84l-43.58,47c-.25.27-.13.72.23.82l16.54,4.72c.39.11.49.62.17.87l-61.6,48.37c-.33.26-.81.02-.81-.39v-15.13c0-.12.05-.24.13-.33l25.44-28.59c.21-.24.15-.61-.13-.77l-15.57-8.65c-.32-.18-.35-.62-.05-.84Z"
          fill="var(--amber)"
        />
        <path
          d="m47.69,107.51l21.33-16.75c.2-.16.48-.14.66.04l61.95,61.95c.31.31.09.85-.35.85h-38.07c-.13,0-.26-.05-.35-.15l-45.21-45.21c-.21-.21-.19-.56.04-.75Z"
          fill="var(--text)"
        />
        <path
          d="m.01,153.11C-.03,106.43.06,59.81.02,13.13c0-.25.18-.45.42-.49l4.91-.79c.56-.09.56-.9,0-.99l-4.9-.75c-.24-.04-.42-.25-.42-.49V.5c0-.28.22-.5.49-.5h35.59c.28,0,.5.22.5.5v47.9s-21.34,21.34-21.34,21.34l-.18,83.37c0,.28-.22.5-.5.5-4.68.01-9.38-.02-14.06,0-.28,0-.5-.22-.5-.5Z"
          fill="var(--text)"
        />
        <path
          d="m21.47,72.87c4.8-4.53,9.5-9.75,14.29-14.34.32-.3.85-.07.85.36,0,31.15,0,63.07,0,94.21,0,.28-.23.51-.51.51-4.79,0-9.48,0-14.27,0-.28,0-.5-.22-.5-.5,0-26.63,0-53.26,0-79.89,0-.14.06-.26.15-.36Z"
          fill="var(--text)"
        />
      </svg>
    </div>
  )
}
