import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'

interface UserProfile {
  id: string
  username: string
  bio: string
  avatar_url: string
  created_at: number
}

export default function Profile() {
  const { userId } = useParams<{ userId: string }>()
  const { user: currentUser, authHeaders } = useAuth()
  const navigate = useNavigate()
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState(false)
  const [bio, setBio] = useState('')
  const [avatarUrl, setAvatarUrl] = useState('')
  const [saving, setSaving] = useState(false)

  const isOwnProfile = currentUser?.id === userId

  useEffect(() => {
    if (!userId) return
    const url = isOwnProfile ? '/api/users/me' : `/api/users/${userId}`
    fetch(url, { headers: isOwnProfile ? authHeaders() : {} })
      .then(async (res) => {
        if (!res.ok) throw new Error('User not found')
        const data = await res.json()
        setProfile(data.user)
        setBio(data.user.bio || '')
        setAvatarUrl(data.user.avatar_url || '')
      })
      .catch(() => setError('User not found'))
  }, [userId, isOwnProfile])

  const handleSave = async () => {
    setSaving(true)
    try {
      const res = await fetch('/api/users/me', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        body: JSON.stringify({ bio, avatar_url: avatarUrl }),
      })
      if (res.ok) {
        const data = await res.json()
        setProfile(data.user)
        setEditing(false)
      }
    } finally {
      setSaving(false)
    }
  }

  if (error) {
    return (
      <div className="profile-page">
        <div className="profile-card">
          <h2 className="profile-error-title">User not found</h2>
          <button className="btn btn-primary" onClick={() => navigate('/')}>Back to Home</button>
        </div>
      </div>
    )
  }

  if (!profile) {
    return (
      <div className="profile-page">
        <div className="profile-card">
          <p className="profile-loading">Loading...</p>
        </div>
      </div>
    )
  }

  const joinDate = new Date(profile.created_at * 1000).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })

  return (
    <div className="profile-page">
      <div className="profile-card">
        <div className="profile-avatar">
          {profile.avatar_url ? (
            <img src={profile.avatar_url} alt={profile.username} />
          ) : (
            <div className="profile-avatar-placeholder">
              {profile.username[0].toUpperCase()}
            </div>
          )}
        </div>

        <h1 className="profile-username">{profile.username}</h1>

        {profile.bio && <p className="profile-bio">{profile.bio}</p>}

        <p className="profile-joined">Joined {joinDate}</p>

        {isOwnProfile && !editing && (
          <button className="btn btn-secondary profile-edit-btn" onClick={() => setEditing(true)}>
            Edit Profile
          </button>
        )}

        {isOwnProfile && editing && (
          <div className="profile-edit-form">
            <textarea
              placeholder="Tell about yourself..."
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              maxLength={200}
            />
            <input
              type="url"
              placeholder="Avatar URL"
              value={avatarUrl}
              onChange={(e) => setAvatarUrl(e.target.value)}
            />
            <div className="profile-edit-actions">
              <button className="btn btn-primary btn-sm" onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save'}
              </button>
              <button className="btn btn-secondary btn-sm" onClick={() => setEditing(false)}>
                Cancel
              </button>
            </div>
          </div>
        )}

        <button className="btn btn-secondary" onClick={() => navigate('/')}>Back to Home</button>
      </div>
    </div>
  )
}
