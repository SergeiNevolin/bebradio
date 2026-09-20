import { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { ACCENT_PRESETS, DEFAULT_ACCENT, applyAccent, getStoredAccent, getStoredTheme, applyTheme } from '../lib/theme'
import styles from './Settings.module.css'

type Tab = 'profile' | 'appearance' | 'admin'

export default function Settings() {
  const { user } = useAuth()
  const [tab, setTab] = useState<Tab>('profile')

  const [dark, setDark] = useState(() => getStoredTheme() === 'dark')

  const [accent, setAccent] = useState(getStoredAccent)
  const [bio, setBio] = useState('')
  const [avatarUrl, setAvatarUrl] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)

  const isAdmin = !!user && user.role === 'admin'
  const [adminQuery, setAdminQuery] = useState('')
  const [adminResults, setAdminResults] = useState<Array<{ id: string; username: string; role: string }>>([])
  const [adminLoading, setAdminLoading] = useState(false)

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

  useEffect(() => {
    if (!adminQuery.trim()) {
      setAdminResults([])
      return
    }
    const t = setTimeout(async () => {
      setAdminLoading(true)
      try {
        const results = await api.adminSearchUsers(adminQuery.trim())
        setAdminResults(results)
      } catch { /* ignore */ }
      setAdminLoading(false)
    }, 200)
    return () => clearTimeout(t)
  }, [adminQuery])

  const handlePromote = async (userId: string) => {
    await api.adminPromote(userId)
    setAdminResults((prev) => prev.map((u) => u.id === userId ? { ...u, role: 'admin' } : u))
  }

  const handleDemote = async (userId: string) => {
    await api.adminDemote(userId)
    setAdminResults((prev) => prev.map((u) => u.id === userId ? { ...u, role: 'user' } : u))
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
            {isAdmin && (
              <button
                className={`${styles.settingsNavItem}${tab === 'admin' ? ` ${styles.settingsNavItemActive}` : ''}`}
                onClick={() => setTab('admin')}
              >
                Admin
              </button>
            )}
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

          {tab === 'admin' && (
            <div className={styles.settingsSection}>
              <h3 className={styles.settingsSectionTitle}>Управление ролями</h3>
              <p className={styles.settingsSectionDesc}>Найдите пользователя по имени и назначьте или снимите права администратора.</p>

              <div className={styles.settingsField}>
                <input
                  className={styles.settingsInput}
                  type="text"
                  placeholder="Имя пользователя..."
                  value={adminQuery}
                  onChange={(e) => setAdminQuery(e.target.value)}
                />
              </div>

              {adminLoading && <p className={styles.settingsHint}>Поиск...</p>}

              {adminResults.length > 0 && (
                <div style={{ marginTop: 12 }}>
                  {adminResults.map((u) => (
                    <div key={u.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 0', borderBottom: '1px solid var(--border)' }}>
                      <div>
                        <span style={{ fontWeight: 600 }}>{u.username}</span>
                        <span style={{ marginLeft: 8, fontSize: 12, color: 'var(--text-muted)' }}>{u.id}</span>
                        {u.role === 'admin' && (
                          <span style={{ marginLeft: 8, fontSize: 11, fontWeight: 700, color: 'var(--primary)', background: 'color-mix(in srgb, var(--primary) 12%, transparent)', padding: '2px 6px', borderRadius: 4 }}>ADMIN</span>
                        )}
                      </div>
                      {u.role === 'admin' ? (
                        <button className="btn btn-secondary btn-sm" onClick={() => handleDemote(u.id)}>Снять админа</button>
                      ) : (
                        <button className="btn btn-sm" onClick={() => handlePromote(u.id)}>Сделать админом</button>
                      )}
                    </div>
                  ))}
                </div>
              )}

              {adminQuery.trim() && !adminLoading && adminResults.length === 0 && (
                <p className={styles.settingsHint}>Пользователи не найдены</p>
              )}
            </div>
          )}

        </div>
      </div>
    </div>
  )
}
