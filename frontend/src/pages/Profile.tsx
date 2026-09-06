import { useParams, useNavigate } from 'react-router-dom'
import ProfileCard from '../components/ProfileCard'
import styles from './Profile.module.css'

export default function Profile() {
  const { userId } = useParams<{ userId: string }>()
  const navigate = useNavigate()

  if (!userId) {
    return (
      <div className={styles.profilePage}>
        <div className={styles.profileCard}>
          <p className={styles.profileErrorTitle}>User not found</p>
          <button className="btn btn-secondary btn-sm" onClick={() => navigate('/')}>Back to Home</button>
        </div>
      </div>
    )
  }

  return (
    <div className={styles.profilePage}>
      <ProfileCard userId={userId} />
    </div>
  )
}
