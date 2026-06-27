package server

import (
	"context"
	"io"

	"github.com/psychic-coder/shardroute/internal/limiter"
	"github.com/psychic-coder/shardroute/internal/metrics"
	pb "github.com/psychic-coder/shardroute/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	pb.UnimplementedRateLimiterServer
	Store  *limiter.RedisStore
	Cache  *limiter.LocalCache
	Fail   *limiter.FailureHandler
	Config limiter.LimitConfig
}

func RegisterGRPC(s *grpc.Server, impl pb.RateLimiterServer) {
	pb.RegisterRateLimiterServer(s, impl)
}

func (g *GRPCServer) Check(ctx context.Context, req *pb.CheckRequest) (*pb.CheckResponse, error) {
	if req.Key == "" || req.Cost < 0 || req.LimitName == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if g.Cache != nil {
		if allowed, rem := g.Cache.CheckLocal(req.Key, float64(req.Cost)); allowed {
			metrics.LocalCacheHit.Inc()
			metrics.RequestsTotal.WithLabelValues("allowed").Inc()
			return &pb.CheckResponse{Allowed: true, TokensRemaining: rem}, nil
		}
		metrics.LocalCacheMiss.Inc()
	}

	res, err := g.Store.CheckAndDecrement(ctx, req.Key, g.Config, float64(req.Cost))
	if err != nil && g.Fail != nil {
		allowed, failErr := g.Fail.HandleRedisError(err, false)
		if failErr != nil {
			return nil, status.Error(codes.Unavailable, failErr.Error())
		}
		res.Allowed = allowed
	} else if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	if res.Allowed {
		metrics.RequestsTotal.WithLabelValues("allowed").Inc()
	} else {
		metrics.RequestsTotal.WithLabelValues("rejected").Inc()
	}

	return &pb.CheckResponse{
		Allowed:         res.Allowed,
		TokensRemaining: res.TokensRemaining,
		RetryAfterMs:    res.RetryAfterMillis,
	}, nil
}

func (g *GRPCServer) StreamCheck(stream pb.RateLimiter_StreamCheckServer) error {
	ctx := stream.Context()
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp, err := g.Check(ctx, req)
		if err != nil {
			resp = &pb.CheckResponse{Allowed: false, Error: err.Error()}
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}
