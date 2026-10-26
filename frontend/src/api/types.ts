export type PageResponse<T> = {
  list: T[]
  has_more: boolean
  next_id: number
}

export type TokenResponse = {
  token: string
  refresh_token: string
}

export type Account = {
  id: number
  username: string
  avatar_url?: string
  bio?: string
}

export type ProfileSummary = Account & {
  video_count: number
  total_likes: number
  follower_count: number
  vlogger_count: number
}

export type Video = {
  id: number
  author_id: number
  username: string
  title: string
  description?: string
  play_url: string
  cover_url: string
  create_time: string
  likes_count: number
  popularity: number
}

export type Comment = {
  id: number
  username: string
  video_id: number
  author_id: number
  content: string
  created_at: string
}

export type FeedAuthor = {
  id: number
  username: string
}

export type FeedVideoItem = {
  id: number
  author: FeedAuthor
  title: string
  description?: string
  play_url: string
  cover_url: string
  create_time: number
  likes_count: number
  is_liked: boolean
}

export type ListLatestResponse = {
  video_list: FeedVideoItem[]
  next_time: number
  has_more: boolean
}

export type ListLikesCountResponse = {
  video_list: FeedVideoItem[]
  next_likes_count?: number
  next_id?: number
  has_more: boolean
}

export type ListByPopularityResponse = {
  video_list: FeedVideoItem[]
  asof: number
  next_offset: number
  has_more: boolean
}

export type ListByFollowingResponse = {
  video_list: FeedVideoItem[]
  next_id: number
  has_more: boolean
}

export type ListByTagResponse = {
  video_list: FeedVideoItem[]
  next_id: number
  has_more: boolean
}

export type IsLikedResponse = {
  is_liked: boolean
}

export type IsFollowedResponse = {
  isFollowed: boolean
}

export type GetAllFollowersResponse = {
  followers: Account[]
  has_more: boolean
  next_id: number
}

export type GetAllVloggersResponse = {
  vloggers: Account[]
  has_more: boolean
  next_id: number
}

export type DirectMessage = {
  id: number
  from_id: number
  to_id: number
  content: string
  is_read: boolean
  created_at: string
}

export type ListMessagesResponse = {
  messages: DirectMessage[]
  has_more: boolean
  next_id: number
}

export type Notification = {
  id: number
  recipient_id: number
  sender_id: number
  username: string
  type: string
  target_id: number
  content: string
  is_read: boolean
  created_at: string
}
