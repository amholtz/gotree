package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

// matrixCmd represents the matrix command
var matrixCmd = &cobra.Command{
	Use:   "matrix",
	Short: "Prints distance matrix associated to the input tree",
	Long:  `Prints distance matrix associated to the input tree.`,
	Run: func(cmd *cobra.Command, args []string) {
		f := openWriteFile(outtreefile)
		for t := range readTrees(intreefile) {
			tips := t.Tree.Tips()
			f.WriteString(fmt.Sprintf("%d\n", len(tips)))
			mat := t.Tree.ToDistanceMatrix()
			for i, t := range tips {
				f.WriteString(t.Name())
				for j, _ := range tips {
					f.WriteString("\t" + fmt.Sprintf("%.12f", mat[i][j]))
				}
				f.WriteString("\n")
			}
		}
		f.Close()
	},
}

func init() {
	RootCmd.AddCommand(matrixCmd)
	matrixCmd.PersistentFlags().StringVarP(&intreefile, "input", "i", "stdin", "Input tree")
	matrixCmd.PersistentFlags().StringVarP(&outtreefile, "output", "o", "stdout", "Matrix output file")
}
