-- KEYS[1] = rate limit key
-- ARGV[1] = current timestamp (millis)
-- ARGV[2] = capacity
-- ARGV[3] = refill rate per second
-- ARGV[4] = cost
-- ARGV[5] = window size millis
-- ARGV[6] = max requests per window

local key = KEYS[1]
local now = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local refill_rate = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])
local window_size = tonumber(ARGV[5])
local max_per_window = tonumber(ARGV[6])

local bucket_key = key .. ":bucket"
local window_key = key .. ":window"

local bucket = redis.call('HMGET', bucket_key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1]) or capacity
local last_refill = tonumber(bucket[2]) or now

local elapsed = math.max(0, now - last_refill)
local refill_amount = (elapsed / 1000.0) * refill_rate
tokens = math.min(capacity, tokens + refill_amount)

redis.call('ZREMRANGEBYSCORE', window_key, 0, now - window_size)
local window_count = redis.call('ZCARD', window_key)

local allowed = 0
local retry_after = 0

if tokens >= cost and window_count < max_per_window then
  tokens = tokens - cost
  redis.call('ZADD', window_key, now, now .. ':' .. math.random())
  redis.call('EXPIRE', window_key, math.ceil(window_size / 1000) + 1)
  allowed = 1
else
  if tokens < cost then
    if refill_rate > 0 then
      retry_after = math.ceil(((cost - tokens) / refill_rate) * 1000)
    else
      retry_after = -1
    end
  else
    retry_after = window_size
  end
end

redis.call('HMSET', bucket_key, 'tokens', tokens, 'last_refill', now)
if refill_rate > 0 then
  redis.call('EXPIRE', bucket_key, math.ceil(capacity / refill_rate) + 1)
end

return {allowed, tostring(tokens), retry_after}
