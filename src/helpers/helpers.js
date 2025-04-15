export function convertCsv(data) {
        if (!data) return []
        if (!data.length) return []

        let names = data.shift()
        let cols = [];
        for (let row of data) {
                let curr = {}
                for (let i = 0; i < names.length; i++) {
                        curr[names[i]] = row[i]
                }

                cols.push(curr)
        }

        return Object.assign(cols, {
                columns: names
        })
}