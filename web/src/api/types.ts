export interface Me {
  id: number
  username: string
  displayName: string
  selfProfileId: number
  familyId: number | null
}

export type Gender = '' | 'male' | 'female'

export interface Profile {
  id: number
  displayName: string
  gender: Gender
  birthYear: number | null
  medicalHistory: string
  medications: string
  isSelf: boolean
  isManaged: boolean
  managerName?: string
  canWrite: boolean
}

export interface ProfileInput {
  displayName: string
  gender: Gender
  birthYear: number | null
  medicalHistory: string
  medications: string
}

export interface Family {
  id: number
  name: string
  inviteCode: string
  createdBy: number
}

export interface FamilyMember {
  userId: number
  username: string
  displayName: string
  isMe: boolean
}

export interface FamilyView {
  family: Family | null
  members: FamilyMember[]
}

export interface ReadingItem {
  systolic: number
  diastolic: number
  pulse: number | null
}

export type Period = 'am' | 'pm' | 'evening'

export interface Reading {
  id: number
  profileId: number
  measuredAt: string
  period: Period
  systolic: number
  diastolic: number
  pulse: number | null
  note: string
  items: ReadingItem[]
}

export interface ReadingInput {
  measuredAt?: string
  items: ReadingItem[]
  note: string
}

export type TrendMode = 'raw' | 'ampm'

export interface TrendPoint {
  at: string
  systolic: number
  diastolic: number
  pulse: number | null
}

export interface TrendBucket {
  date: string
  period: Period
  systolic: number | null
  diastolic: number | null
  pulse: number | null
  count: number
  systolicSum: number
  diastolicSum: number
  pulseSum: number
  pulseCount: number
}

export interface TrendSummary {
  count: number
  systolic: number | null
  diastolic: number | null
  pulse: number | null
}

export interface Trend {
  mode: TrendMode
  from: string
  to: string
  points?: TrendPoint[]
  buckets?: TrendBucket[]
  summary: TrendSummary
}
