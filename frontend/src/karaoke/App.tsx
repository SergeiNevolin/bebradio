import { AnimatePresence, motion } from 'framer-motion'
import { useEffect, useState } from 'react'
import Catalog from './components/Catalog'
import Player from './components/Player'
import { AuthRequiredError } from './lib/api'
import { loadManifest } from './lib/songs'
import { useKaraoke } from './store'

const LOAD_FAILED = 'Не удалось загрузить каталог — проверьте сеть или запустите бэкенд'

export default function App() {
  const screen = useKaraoke((s) => s.screen)
  const songs = useKaraoke((s) => s.songs)
  const setSongs = useKaraoke((s) => s.setSongs)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    loadManifest()
      .then(setSongs)
      .catch((e: unknown) => setLoadError(e instanceof AuthRequiredError ? e.message : LOAD_FAILED))
  }, [setSongs])

  return (
    <div className="flex h-full flex-col bg-bg">
      {loadError && songs.length === 0 && (
        <p className="w-full px-10 pb-2 text-[13px] text-danger max-sm:px-3.5">{loadError}</p>
      )}

      <main className="min-h-0 flex-1">
        <AnimatePresence mode="wait">
          <motion.div
            key={screen}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.22 }}
            className="h-full"
          >
            {screen === 'catalog' ? <Catalog /> : <Player />}
          </motion.div>
        </AnimatePresence>
      </main>
    </div>
  )
}
