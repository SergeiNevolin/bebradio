import { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { ACCENT_PRESETS, DEFAULT_ACCENT, applyAccent, getStoredAccent, getStoredTheme, applyTheme } from '../lib/theme'
import styles from './Settings.module.css'

type Tab = 'profile' | 'appearance'

export default function Settings() {
  const { user } = useAuth()
  const [tab, setTab] = useState<Tab>('profile')

  const [dark, setDark] = useState(() => getStoredTheme() === 'dark')

  const [accent, setAccent] = useState(getStoredAccent)
  const [bio, setBio] = useState('')
  const [avatarUrl, setAvatarUrl] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    applyTheme(dark ? 'dark' : 'light')
  }, [dark])

  useEffect(() => {
    applyAccent(accent)
  }, [accent])

  useEffect(() => {
    if (!user) return
    api.getMeProfile()
      .then((data) => {
        setBio(data.user.bio || '')
        setAvatarUrl(data.user.avatar_url || '')
      })
      .catch(() => {})
  }, [user])

  const handleSaveProfile = async () => {
    setSaving(true)
    setSaved(false)
    try {
      await api.updateMeProfile({ bio, avatar_url: avatarUrl })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className={styles.settingsPage}>
      <div className={styles.settingsLayout}>
        <nav className={styles.settingsSidebar}>
          <h2 className={styles.settingsSidebarTitle}>Settings</h2>
          <div className={styles.settingsNav}>
            <button
              className={`${styles.settingsNavItem}${tab === 'profile' ? ` ${styles.settingsNavItemActive}` : ''}`}
              onClick={() => setTab('profile')}
            >
              Profile
            </button>
            <button
              className={`${styles.settingsNavItem}${tab === 'appearance' ? ` ${styles.settingsNavItemActive}` : ''}`}
              onClick={() => setTab('appearance')}
            >
              Appearance
            </button>
          </div>
        </nav>

        <div className={styles.settingsContent}>
          {tab === 'profile' && (
            <>
              <div className={styles.settingsSection}>
                <h3 className={styles.settingsSectionTitle}>Public profile</h3>
                <p className={styles.settingsSectionDesc}>This information will be displayed on your profile.</p>

                <div className={styles.settingsField}>
                  <label className={styles.settingsLabel}>Bio</label>
                  <textarea
                    className={styles.settingsTextarea}
                    placeholder="Tell about yourself..."
                    value={bio}
                    onChange={(e) => setBio(e.target.value)}
                    maxLength={200}
                  />
                  <p className={styles.settingsHint}>{bio.length}/200</p>
                </div>

                <div className={styles.settingsField}>
                  <label className={styles.settingsLabel}>Avatar URL</label>
                  <input
                    className={styles.settingsInput}
                    type="url"
                    placeholder="https://example.com/avatar.png"
                    value={avatarUrl}
                    onChange={(e) => setAvatarUrl(e.target.value)}
                  />
                </div>

                <div className={styles.settingsActions}>
                  <button className="btn btn-primary btn-sm" onClick={handleSaveProfile} disabled={saving}>
                    {saving ? 'Saving...' : 'Save profile'}
                  </button>
                  {saved && <span className={styles.settingsSaved}>Saved</span>}
                </div>
              </div>
            </>
          )}

          {tab === 'appearance' && (
            <>
              <div className={styles.settingsSection}>
                <h3 className={styles.settingsSectionTitle}>Theme</h3>
                <p className={styles.settingsSectionDesc}>Switch between light and dark mode.</p>

                <label className="settings-toggle">
                  <input
                    type="checkbox"
                    checked={dark}
                    onChange={(e) => setDark(e.target.checked)}
                  />
                  <span className="toggle-slider"></span>
                  <span className="toggle-label">Dark mode</span>
                </label>
              </div>

              <div className={styles.settingsSection}>
                <h3 className={styles.settingsSectionTitle}>Accent color</h3>
                <p className={styles.settingsSectionDesc}>Customize the accent color of the interface.</p>

                <div className={styles.settingsAccentGrid}>
                  {ACCENT_PRESETS.map((preset) => (
                    <button
                      key={preset.name}
                      type="button"
                      className={`${styles.settingsAccentDot}${accent === preset.value ? ` ${styles.settingsAccentDotActive}` : ''}`}
                      style={{ background: preset.value || DEFAULT_ACCENT }}
                      title={preset.name}
                      onClick={() => setAccent(preset.value)}
                    />
                  ))}
                </div>
              </div>
            </>
          )}

        </div>
      </div>
    </div>
  )
}
