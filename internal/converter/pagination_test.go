package converter

import (
	"testing"

	pb "github.com/kazuto/parlance-server/gen/parlance/v1"
)

func TestPaginationToProto(t *testing.T) {
	tests := []struct {
		name     string
		total    int64
		page     int32
		perPage  int32
		expected *pb.PaginationResponse
	}{
		{
			name:    "first page with results",
			total:   100,
			page:    1,
			perPage: 10,
			expected: &pb.PaginationResponse{
				Total:      100,
				Page:       1,
				PerPage:    10,
				TotalPages: 10,
			},
		},
		{
			name:    "middle page",
			total:   100,
			page:    5,
			perPage: 10,
			expected: &pb.PaginationResponse{
				Total:      100,
				Page:       5,
				PerPage:    10,
				TotalPages: 10,
			},
		},
		{
			name:    "last page",
			total:   95,
			page:    10,
			perPage: 10,
			expected: &pb.PaginationResponse{
				Total:      95,
				Page:       10,
				PerPage:    10,
				TotalPages: 10,
			},
		},
		{
			name:    "partial last page",
			total:   25,
			page:    3,
			perPage: 10,
			expected: &pb.PaginationResponse{
				Total:      25,
				Page:       3,
				PerPage:    10,
				TotalPages: 3,
			},
		},
		{
			name:    "no results",
			total:   0,
			page:    1,
			perPage: 10,
			expected: &pb.PaginationResponse{
				Total:      0,
				Page:       1,
				PerPage:    10,
				TotalPages: 1, // Minimum 1 page even with no results
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PaginationToProto(tt.total, tt.page, tt.perPage)

			if result.Total != tt.expected.Total {
				t.Errorf("Total = %v, want %v", result.Total, tt.expected.Total)
			}
			if result.Page != tt.expected.Page {
				t.Errorf("Page = %v, want %v", result.Page, tt.expected.Page)
			}
			if result.PerPage != tt.expected.PerPage {
				t.Errorf("PerPage = %v, want %v", result.PerPage, tt.expected.PerPage)
			}
			if result.TotalPages != tt.expected.TotalPages {
				t.Errorf("TotalPages = %v, want %v", result.TotalPages, tt.expected.TotalPages)
			}
		})
	}
}
