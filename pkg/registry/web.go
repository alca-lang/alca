// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	_ "embed"
	"net/http"
)

// LogoSVG is the official Alca vector logo.
const LogoSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="0 0 1254 1254">
<rect width="1254" height="1254" rx="200" fill="#006dad"/>
<path d="M 772 906 L 793 906 L 836 848 L 879 906 L 900 906 L 894 899 L 892 895 L 862 855 L 860 851 L 845 832 L 843 828 L 829 828 L 826 833 L 822 837 L 820 841 L 814 848 L 812 852 L 806 859 L 804 863 L 795 874 L 793 878 Z M 653 834 L 649 841 L 649 893 L 650 896 L 655 902 L 663 906 L 740 906 L 740 890 L 671 890 L 669 889 L 666 885 L 666 849 L 669 845 L 671 844 L 740 844 L 740 828 L 664 828 L 657 831 Z M 524 828 L 524 906 L 609 906 L 609 890 L 543 890 L 542 889 L 542 828 Z M 354 906 L 375 906 L 388 887 L 417 849 L 421 852 L 460 906 L 481 906 L 473 894 L 467 887 L 465 883 L 462 880 L 460 876 L 424 828 L 411 828 L 409 830 Z M 536 631 L 660 631 L 725 737 L 774 737 L 688 596 L 562 595 L 557 598 Z M 626 339 L 387 721 L 435 721 L 627 426 L 831 764 L 880 764 Z" fill="#ffffff" fill-rule="evenodd"/>
</svg>`

//go:embed index.html
var IndexHTML string

// handleIndex serves the single-page web UI.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(IndexHTML))
}

// handleLogo serves the vector SVG logo.
func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(LogoSVG))
}
