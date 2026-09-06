import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import styles from '../pages/Profile.module.css'

interface UserProfile {
  id: string
  username: string
  bio: string
  avatar_url: string
  created_at: number
}

interface ProfileCardProps {
  userId: string
  // Called right before an internal navigation happens (e.g. "Edit profile"),
  // so a host modal can close itself first.
  onNavigate?: () => void
}

export default function ProfileCard({ userId, onNavigate }: ProfileCardProps) {
  const { user: currentUser } = useAuth()
  const navigate = useNavigate()
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [error, setError] = useState('')

  const isOwnProfile = currentUser?.id === userId

  useEffect(() => {
    if (!userId) return
    let cancelled = false
    setProfile(null)
    setError('')
    const fetchProfile = isOwnProfile ? api.getMeProfile() : api.getUser(userId)
    fetchProfile
      .then((data) => {
        if (!cancelled) setProfile(data.user as unknown as UserProfile)
      })
      .catch(() => {
        if (!cancelled) setError('User not found')
      })
    return () => {
      cancelled = true
    }
  }, [userId, isOwnProfile])

  if (error) {
    return (
      <div className={styles.profileCard}>
        <p className={styles.profileErrorTitle}>User not found</p>
      </div>
    )
  }

  if (!profile) {
    return (
      <div className={styles.profileCard}>
        <p className={styles.profileLoading}>Loading...</p>
      </div>
    )
  }

  const joinDate = new Date(profile.created_at * 1000).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })

  const goToSettings = () => {
    onNavigate?.()
    navigate('/settings')
  }

  return (
    <div className={styles.profileCard}>
      <div className={styles.profileAvatar}>
        {profile.avatar_url ? (
          <img src={profile.avatar_url} alt={profile.username} />
        ) : (
          <div className={styles.profileAvatarPlaceholder}>
            {profile.username[0].toUpperCase()}
          </div>
        )}
      </div>

      <h1 className={styles.profileName}>{profile.username}</h1>

      {profile.bio && <p className={styles.profileBio}>{profile.bio}</p>}

      <div className={styles.profileMeta}>
        <span>Joined {joinDate}</span>
      </div>

      {isOwnProfile && (
        <button className="btn btn-secondary btn-sm" onClick={goToSettings}>
          Edit profile
        </button>
      )}
    </div>
  )
}
