package tencentcloud

import "sort"

//isExist check string in strArray?
func isExist(target string, strArray []string) bool {
	//调用前先对目标数组自行排序，在这里面排序会改变数组的顺序，可能带来意想不到的结果
	//sort.Strings(strArray)
	index := sort.SearchStrings(strArray, target)
	if index < len(strArray) && strArray[index] == target {
		return true
	}
	return false
}
