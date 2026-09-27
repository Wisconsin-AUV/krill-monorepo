import React from 'react'
import {
  Pagination,
  PaginationGap,
  PaginationList,
  PaginationNext,
  PaginationPage,
  PaginationPrevious,
} from './Pagination'

interface PaginationControlsProps {
  totalPages: number
  currentPage: number
  onPageChange: (page: number) => void
  className?: string
}

/**
 * Generates an array of page numbers to display, with gaps represented as null
 */
function generatePageNumbers(currentPage: number, totalPages: number): (number | null)[] {
  if (totalPages <= 0) {
    return []
  }

  const pages: (number | null)[] = []
  const maxVisible = 7 // Show up to 7 page numbers

  if (totalPages <= maxVisible) {
    // Show all pages if total is small
    for (let i = 1; i <= totalPages; i++) {
      pages.push(i)
    }
  } else {
    // Always show first page
    pages.push(1)

    // Calculate start and end of visible range around current page
    let start = Math.max(2, currentPage - 1)
    let end = Math.min(totalPages - 1, currentPage + 1)

    // Adjust if we're near the beginning
    if (currentPage <= 3) {
      end = Math.min(5, totalPages - 1)
    }

    // Adjust if we're near the end
    if (currentPage >= totalPages - 2) {
      start = Math.max(2, totalPages - 4)
    }

    // Add gap if needed before the range
    if (start > 2) {
      pages.push(null) // gap
    }

    // Add pages in the range
    for (let i = start; i <= end; i++) {
      pages.push(i)
    }

    // Add gap if needed after the range
    if (end < totalPages - 1) {
      pages.push(null) // gap
    }

    // Always show last page
    pages.push(totalPages)
  }

  return pages
}

export const PaginationControls: React.FC<PaginationControlsProps> = ({
  totalPages,
  currentPage,
  onPageChange,
  className,
}) => {
  const pageNumbers = generatePageNumbers(currentPage, totalPages)

  if (totalPages <= 1) {
    return null // Don't show pagination if there's only one page or no pages
  }

  const handlePageClick = (e: React.MouseEvent, page: number) => {
    e.preventDefault()
    onPageChange(page)
  }

  return (
    <Pagination className={className}>
      <PaginationPrevious
        to={null}
        onClick={
          currentPage > 1 ? (e: React.MouseEvent) => handlePageClick(e, currentPage - 1) : undefined
        }
      />
      <PaginationList>
        {pageNumbers.map((pageNum, index) => {
          if (pageNum === null) {
            return <PaginationGap key={`gap-${index}`} />
          }
          return (
            <PaginationPage
              key={pageNum}
              current={pageNum === currentPage}
              onClick={(e: React.MouseEvent) => {
                e.preventDefault()
                onPageChange(pageNum)
              }}
            >
              {pageNum.toString()}
            </PaginationPage>
          )
        })}
      </PaginationList>
      <PaginationNext
        to={null}
        onClick={
          currentPage < totalPages
            ? (e: React.MouseEvent) => handlePageClick(e, currentPage + 1)
            : undefined
        }
      />
    </Pagination>
  )
}
