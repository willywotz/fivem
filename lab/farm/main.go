package main

/*
#cgo CXXFLAGS:   --std=c++11
#cgo CPPFLAGS:   -I./include
#cgo LDFLAGS:    -L./lib -lopencv_stereo4110 -lopencv_tracking4110 -lopencv_superres4110 -lopencv_stitching4110 -lopencv_optflow4110 -lopencv_gapi4110 -lopencv_face4110 -lopencv_dpm4110 -lopencv_dnn_objdetect4110 -lopencv_ccalib4110 -lopencv_bioinspired4110 -lopencv_bgsegm4110 -lopencv_aruco4110 -lopencv_xobjdetect4110 -lopencv_ximgproc4110 -lopencv_xfeatures2d4110 -lopencv_videostab4110 -lopencv_video4110 -lopencv_structured_light4110 -lopencv_shape4110 -lopencv_rgbd4110 -lopencv_rapid4110 -lopencv_objdetect4110 -lopencv_mcc4110 -lopencv_highgui4110 -lopencv_datasets4110 -lopencv_calib3d4110 -lopencv_videoio4110 -lopencv_text4110 -lopencv_line_descriptor4110 -lopencv_imgcodecs4110 -lopencv_img_hash4110 -lopencv_hfs4110 -lopencv_fuzzy4110 -lopencv_features2d4110 -lopencv_dnn_superres4110 -lopencv_dnn4110 -lopencv_xphoto4110 -lopencv_wechat_qrcode4110 -lopencv_surface_matching4110 -lopencv_reg4110 -lopencv_quality4110 -lopencv_plot4110 -lopencv_photo4110 -lopencv_phase_unwrapping4110 -lopencv_ml4110 -lopencv_intensity_transform4110 -lopencv_imgproc4110 -lopencv_flann4110 -lopencv_core4110 -lade -lquirc -llibprotobuf -lIlmImf -llibpng -llibopenjp2 -llibwebp -llibtiff -llibjpeg-turbo -lzlib -lkernel32 -lgdi32 -lwinspool -lshell32 -lole32 -loleaut32 -luuid -lcomdlg32 -ladvapi32 -luser32

#include "opencv2/opencv.hpp"
*/
import "C"

var _ C.Mat // Ensure C.Mat is used to avoid unused import error

func main() {}

// import (
// 	"fmt"
// 	"image"
// 	"image/color"

// 	"gocv.io/x/gocv"
// )

// func main() {
// 	// Load image
// 	img := gocv.IMRead("image1.png", gocv.IMReadColor)
// 	if img.Empty() {
// 		fmt.Println("Error reading image")
// 		return
// 	}

// 	// 1. Edge Detection
// 	edges := gocv.NewMat()
// 	gocv.Canny(img, &edges, 50, 150)

// 	// 2. Line Detection
// 	lines := gocv.NewMat()
// 	gocv.HoughLinesP(edges, &lines, 1, float64(gocv.DegreeToRadians(1)), 80, 50, 10)

// 	// 3. Node Detection (HoughCircles for round nodes)
// 	circles := gocv.NewMat()
// 	gocv.HoughCirclesWithParams(img, &circles, gocv.HoughGradient, 1, 20, 50, 30, 5, 30)

// 	// 4. Parse detected nodes and edges
// 	nodes := make([]image.Point, circles.Cols())
// 	for i := 0; i < circles.Cols(); i++ {
// 		v := circles.GetVecfAt(0, i)
// 		nodes[i] = image.Pt(int(v[0]), int(v[1]))
// 		// Draw node for visualization (optional)
// 		gocv.Circle(&img, nodes[i], int(v[2]), color.RGBA{0, 255, 0, 0}, 2)
// 	}

// 	edgesList := []struct{ A, B image.Point }{}
// 	for i := 0; i < lines.Rows(); i++ {
// 		l := lines.Row(i)
// 		x1 := l.GetIntAt(0, 0)
// 		y1 := l.GetIntAt(0, 1)
// 		x2 := l.GetIntAt(0, 2)
// 		y2 := l.GetIntAt(0, 3)
// 		edgesList = append(edgesList, struct{ A, B image.Point }{image.Pt(x1, y1), image.Pt(x2, y2)})
// 		// Draw edge for visualization (optional)
// 		gocv.Line(&img, image.Pt(x1, y1), image.Pt(x2, y2), color.RGBA{255, 0, 0, 0}, 1)
// 	}

// 	// 5. Save/Display Result
// 	gocv.IMWrite("result.png", img)

// 	// 6. Build and print graph structure
// 	fmt.Println("Nodes:", nodes)
// 	fmt.Println("Edges:", edgesList)
// }
