package converter

import (
	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

// PaginationToProto converts pagination info to protobuf response
func PaginationToProto(total int64, page, perPage int32) *pb.PaginationResponse {
	totalPages := int32((total + int64(perPage) - 1) / int64(perPage))
	if totalPages < 1 {
		totalPages = 1
	}

	return &pb.PaginationResponse{
		Total:      int32(total),
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}
}

// GetPaginationParams extracts pagination parameters with defaults
func GetPaginationParams(req *pb.PaginationRequest) (page, perPage int32) {
	page = 1
	perPage = 20

	if req != nil {
		if req.Page > 0 {
			page = req.Page
		}
		if req.PerPage > 0 {
			perPage = req.PerPage
		}
		// Cap max per page at 100
		if perPage > 100 {
			perPage = 100
		}
	}

	return page, perPage
}
