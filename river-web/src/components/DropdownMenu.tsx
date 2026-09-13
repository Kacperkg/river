import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
// Callers import DropdownMenu.module.css directly for the shared menu-item
// classes (.item, .itemDanger) so their <Link>/<button>/<a> items read
// identically. It isn't re-exported here to keep this a components-only module
// (react-refresh/only-export-components).
import styles from './DropdownMenu.module.css'

type Coords = { top?: number; bottom?: number; right: number }

// TriggerArgs is handed to the trigger render-prop. Spread nothing magic: attach
// `ref` to the trigger button and wire `onClick` to `toggle`; use `open` to
// drive aria-expanded.
interface TriggerArgs {
  ref: React.Ref<HTMLButtonElement>
  toggle: () => void
  open: boolean
}

interface Props {
  // trigger renders the button that opens the menu. It receives a ref (attach to
  // the button), a toggle handler, and the current open state for aria.
  trigger: (args: TriggerArgs) => ReactNode
  // children renders the menu items. `close` is passed so button/link items can
  // dismiss the menu when activated.
  children: (close: () => void) => ReactNode
  // menuLabel is the accessible name for the menu container (role="menu").
  menuLabel?: string
}

// DropdownMenu is a portal-rendered dropdown anchored to a caller-supplied
// trigger. It escapes ancestor overflow clipping via fixed positioning, flips
// above the trigger when there isn't room below, and closes on outside click,
// scroll/resize, and Escape (returning focus to the trigger). On open it moves
// focus to the first menu item so the menu is keyboard-navigable.
//
// The trigger and items are supplied by the caller (render props) so the same
// mechanics back both the episode "⋯" menu and the movie Play split button.
export function DropdownMenu({ trigger, children, menuLabel }: Props) {
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState<Coords | null>(null)
  // positioned gates visibility until the layout effect has measured the menu
  // and decided whether to flip, avoiding a one-frame jump when flipping up.
  const [positioned, setPositioned] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  const close = () => setOpen(false)

  const openMenu = () => {
    const el = triggerRef.current
    if (!el) return
    const r = el.getBoundingClientRect()
    // Provisional placement below the trigger; the layout effect corrects it.
    setCoords({ top: r.bottom + 6, right: window.innerWidth - r.right })
    setPositioned(false)
    setOpen(true)
  }

  const toggle = () => (open ? close() : openMenu())

  // Measure the rendered menu and flip above the trigger when it would overflow
  // the viewport bottom, then focus the first item. Runs before paint.
  useLayoutEffect(() => {
    if (!open) return
    const t = triggerRef.current
    const m = menuRef.current
    if (!t || !m) return
    const r = t.getBoundingClientRect()
    const h = m.offsetHeight
    const right = window.innerWidth - r.right
    const spaceBelow = window.innerHeight - r.bottom
    if (spaceBelow < h + 12 && r.top > h + 12) {
      setCoords({ bottom: window.innerHeight - r.top + 6, right })
    } else {
      setCoords({ top: r.bottom + 6, right })
    }
    setPositioned(true)
    m.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
  }, [open])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (triggerRef.current?.contains(t) || menuRef.current?.contains(t)) return
      setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false)
        triggerRef.current?.focus()
      }
    }
    // Fixed coords go stale on scroll/resize — closing is the simplest correct
    // behaviour (matches how most menus behave when the page moves under them).
    const onMove = () => setOpen(false)
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    window.addEventListener('scroll', onMove, true)
    window.addEventListener('resize', onMove)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('scroll', onMove, true)
      window.removeEventListener('resize', onMove)
    }
  }, [open])

  return (
    <>
      {trigger({ ref: triggerRef, toggle, open })}
      {open && coords && createPortal(
        <div
          ref={menuRef}
          className={styles.menu}
          role="menu"
          aria-label={menuLabel}
          style={{ top: coords.top, bottom: coords.bottom, right: coords.right, visibility: positioned ? 'visible' : 'hidden' }}
        >
          {children(close)}
        </div>,
        document.body,
      )}
    </>
  )
}
