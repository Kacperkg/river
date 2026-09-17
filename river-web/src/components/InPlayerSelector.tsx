import { useEffect, useState } from 'react'
import { RiArrowLeftLine, RiArrowRightSLine } from 'react-icons/ri'
import { api, type Episode, type Season, type TVShow } from '../api'
import styles from './InPlayerSelector.module.css'

interface Props {
  showId: string
  currentSeasonId: string
  currentEpisodeId: string
  onSelect: (seasonId: string, episodeId: string) => void
}

function durationLabel(minutes: number): string | null {
  if (minutes <= 0) return null
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return hours ? `${hours}h ${rest}m` : `${rest}m`
}

export function InPlayerSelector({
  showId, currentSeasonId, currentEpisodeId, onSelect,
}: Props) {
  const [show, setShow] = useState<TVShow | null>(null)
  const [seasons, setSeasons] = useState<Season[]>([])
  const [selectedSeasonId, setSelectedSeasonId] = useState(currentSeasonId)
  const [view, setView] = useState<'episodes' | 'seasons'>('episodes')
  const [episodes, setEpisodes] = useState<Episode[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  const [seasonError, setSeasonError] = useState(false)

  useEffect(() => {
    let active = true
    api.getTVShow(showId)
      .then(nextShow => { if (active) setShow(nextShow) })
      .catch(() => {})
    api.listSeasons(showId)
      .then(nextSeasons => {
        if (active) setSeasons([...nextSeasons].sort((a, b) => a.number - b.number))
      })
      .catch(() => { if (active) setSeasonError(true) })
    return () => { active = false }
  }, [showId])

  useEffect(() => {
    if (view !== 'episodes') return
    let active = true
    // eslint-disable-next-line react-hooks/set-state-in-effect -- loading state belongs to this request
    setLoading(true)
    setError(false)
    api.listEpisodes(showId, selectedSeasonId)
      .then(items => {
        if (!active) return
        setEpisodes([...items].sort((a, b) => a.number - b.number))
      })
      .catch(() => { if (active) setError(true) })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [showId, selectedSeasonId, view])

  const selectedSeason = seasons.find(season => season.id === selectedSeasonId)
  const selectedSeasonName = selectedSeason?.title ||
    (selectedSeason?.number === 0 ? 'Specials' : selectedSeason ? `Season ${selectedSeason.number}` : 'Episodes')

  return (
    <section
      id="episode-selector-panel"
      className={styles['in-player-selector']}
      aria-label="Episodes"
      onMouseLeave={() => {
        setSelectedSeasonId(currentSeasonId)
        setView('episodes')
      }}
      onClick={event => event.stopPropagation()}
      onDoubleClick={event => event.stopPropagation()}
    >
      <div className={styles.episodeSelectorHeader}>
        <span className={styles.episodeSelectorEyebrow}>{show?.title || 'Episodes'}</span>
        <div className={styles.episodeSelectorHeadingRow}>
          {seasons.length > 1 && view === 'episodes' && (
            <button
              type="button"
              className={styles.episodeSelectorBack}
              aria-label="Choose season"
              onClick={() => setView('seasons')}
            >
              <RiArrowLeftLine size={20} aria-hidden="true" />
            </button>
          )}
          <h2 className={styles.episodeSelectorTitle}>
            {view === 'episodes' ? selectedSeasonName : 'Seasons'}
          </h2>
        </div>
      </div>

      <div className={styles.episodeSelectorList}>
        {view === 'seasons' && seasons.map(season => (
          <button
            key={season.id}
            type="button"
            className={`${styles.episodeSeasonItem} ${season.id === selectedSeasonId ? styles.episodeSeasonSelected : ''}`}
            onClick={() => {
              setSelectedSeasonId(season.id)
              setLoading(true)
              setView('episodes')
            }}
          >
            <span>{season.title || (season.number === 0 ? 'Specials' : `Season ${season.number}`)}</span>
            <RiArrowRightSLine size={20} aria-hidden="true" />
          </button>
        ))}
        {view === 'seasons' && seasons.length === 0 && (
          <p className={styles.episodeSelectorMessage}>
            {seasonError ? 'Could not load seasons.' : 'Loading seasons…'}
          </p>
        )}
        {view === 'episodes' && loading && <p className={styles.episodeSelectorMessage}>Loading episodes…</p>}
        {view === 'episodes' && !loading && error && <p className={styles.episodeSelectorMessage}>Could not load episodes.</p>}
        {view === 'episodes' && !loading && !error && episodes.length === 0 && (
          <p className={styles.episodeSelectorMessage}>No episodes in this season.</p>
        )}
        {view === 'episodes' && !loading && !error && episodes.map(episode => {
          const current = episode.id === currentEpisodeId
          return (
            <button
              key={episode.id}
              type="button"
              className={`${styles.episodeSelectorItem} ${current ? styles.episodeSelectorCurrent : ''}`}
              aria-current={current ? 'true' : undefined}
              onClick={() => onSelect(selectedSeasonId, episode.id)}
            >
              <span className={styles.episodeSelectorInfo}>
                <span className={styles.episodeSelectorItemTop}>
                  <span className={styles.episodeSelectorItemTitle}>
                    {episode.is_special ? 'Special' : `${episode.number}.`} {episode.title || `Episode ${episode.number}`}
                  </span>
                  {durationLabel(episode.runtime) && (
                    <span className={styles.episodeSelectorDuration}>{durationLabel(episode.runtime)}</span>
                  )}
                </span>
                {current && <span className={styles.episodeSelectorNowPlaying}>Now playing</span>}
                {episode.description && (
                  <span className={styles.episodeSelectorDescription}>{episode.description}</span>
                )}
              </span>
            </button>
          )
        })}
      </div>
    </section>
  )
}
