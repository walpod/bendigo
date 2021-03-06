package cubic

import "github.com/walpod/bendigo"

type NaturalVertBuilder struct {
	HermiteVertBuilder
}

func NewNaturalVertBuilder(tknots []float64, vertices ...*EnexVertex) *NaturalVertBuilder {
	sb := &NaturalVertBuilder{
		HermiteVertBuilder: *NewHermiteVertBuilder(tknots, vertices...)}
	sb.CalcTangents()
	return sb
}

func (sb *NaturalVertBuilder) AddVertex(knotNo int, vertex bendigo.Vertex) (err error) {
	err = sb.HermiteVertBuilder.AddVertex(knotNo, vertex)
	if err == nil {
		sb.CalcTangents()
	}
	return err
}

func (sb *NaturalVertBuilder) UpdateVertex(knotNo int, vertex bendigo.Vertex) (err error) {
	err = sb.HermiteVertBuilder.UpdateVertex(knotNo, vertex)
	if err == nil {
		sb.CalcTangents()
	}
	return err
}

func (sb *NaturalVertBuilder) DeleteVertex(knotNo int) (err error) {
	err = sb.HermiteVertBuilder.DeleteVertex(knotNo)
	if err == nil {
		sb.CalcTangents()
	}
	return err
}

// CalcTangents calculates and sets the tangent controls of the hermite vertices for natural spline
// mathematical background can be found in "Interpolating Cubic Splines" - 9 (Gary D. Knott) and in
// "An Introduction to Splines for use in Computer Graphics and Geometric Modeling" - 3.1 (Bartels, Beatty, Barsky)
func (sb *NaturalVertBuilder) CalcTangents() {
	n := len(sb.vertices)
	if n < 2 {
		return
	}
	dim := sb.vertices[0].loc.Dim()

	// solve n linear equations of one dimension for given points and return tangents
	var solve func(p []float64) []float64

	if sb.knots.IsUniform() {
		// uniform, solve equations for m[0] ... m[n-1] (A*m = p)
		// 2 1			= 3 * (p1 - p0)
		// 1 4 1		= 3 * (p2 - p0)
		//   1 4 1		= 3 * (p3 - p1)
		//  	...		= ...
		//		  1 4 1 = 3 * (p(n-1) - p(n-3))
		//			1 2 = 3 * (p(n-1) - p(n-2))
		// first transform to upper-diagonal matrix: eliminate 1's below diagonal and convert diagonal to r[i]
		// followed by a back-substitution to yield m[i]
		solve = func(p []float64) []float64 {
			r := make([]float64, n) // diagonal values
			m := make([]float64, n) // resulting tangents

			// forward elimination
			r[0] = 2
			m[0] = 3 * (p[1] - p[0])
			for i := 1; i < n-1; i++ {
				scl := 1 / r[i-1] // factor to scale line above
				r[i] = 4 - scl
				m[i] = 3*(p[i+1]-p[i-1]) - scl*m[i-1]
			}
			scl := 1 / r[n-2]
			r[n-1] = 2 - scl
			m[n-1] = 3*(p[n-1]-p[n-2]) - scl*m[n-2]

			// backward substitution
			m[n-1] /= r[n-1]
			for i := n - 2; i >= 0; i-- {
				m[i] = (m[i] - m[i+1]) / r[i]
			}

			return m
		}
	} else {
		// prepare length of segments
		t := make([]float64, n)
		for i := 0; i < n-1; i++ {
			//t[i] = knots[i+1] - knots[i]
			t[i], _ = sb.knots.SegmentLen(i)
		}

		// non-uniform, solve equations
		// 2	1           ...             = 3 * (p1 - p0) / t0
		// t1	2*(t1+t0)   t0       ...    = 3 * (t0/t1 * p2 + (t1/t0 - t0/t1) * p1 - t1/t0 * p0)
		//      t2          2*(t2+t1)   t1  = ...
		//                  ...
		//                  1           2   = 3 * (p(n-1) - p(n-2)) / t(n-2)
		solve = func(p []float64) []float64 {
			r := make([]float64, n) // diagonal values
			m := make([]float64, n) // resulting tangents

			// forward elimination
			r[0] = 2
			m[0] = 3 * (p[1] - p[0]) / t[0]
			for i := 1; i < n-1; i++ {
				scl := t[i] / r[i-1] // factor to scale line above
				s := t[i] / t[i-1]
				r[i] = 2 * (t[i] + t[i-1])
				if i == 1 {
					r[i] -= scl
				} else {
					r[i] -= scl * t[i-2]
				}
				m[i] = 3 * (p[i+1]/s + (s-1/s)*p[i] - s*p[i-1])
				m[i] -= scl * m[i-1]
			}
			scl := 1 / r[n-2]
			r[n-1] = 2 - scl
			m[n-1] = 3 * (p[n-1] - p[n-2]) / t[n-2]
			m[n-1] -= scl * m[n-2]

			// backward substitution
			m[n-1] /= r[n-1]
			for i := n - 2; i >= 1; i-- {
				m[i] = (m[i] - m[i+1]*t[i-1]) / r[i]
			}
			m[0] = (m[0] - m[1]) / r[0]

			return m
		}
	}

	// prepare empty tangents for all segments
	for i := 0; i < n; i++ {
		sb.vertices[i].entry = bendigo.NewZeroVec(dim)
		sb.vertices[i].exit = sb.vertices[i].entry // TODO or clone ?
	}

	// solve per dimension
	for d := 0; d < dim; d++ {
		// prepare intermediate slices of vertices
		vertsd := make([]float64, n) // TODO rename
		for i := 0; i < n; i++ {
			vertsd[i] = sb.vertices[i].loc[d]
		}

		// solve linear equations to find tangents
		tansd := solve(vertsd)

		// write intermediate result to vertices
		for i := 0; i < n; i++ {
			sb.vertices[i].entry[d] = tansd[i]
			sb.vertices[i].exit[d] = tansd[i]
		}
	}
}
