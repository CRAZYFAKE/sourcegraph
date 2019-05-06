import { quoteIfNeeded } from '../search'

export function queryFindAndReplaceOptions(query: string): { find: string; replace: string } {
    // TODO!(sqs): hacky
    const m = query.match(/^(.*) replace:['"]?(.*?)['"]?$/)
    if (!m) {
        return { find: '', replace: '' }
    }
    const find = m[1]
        .split(/\s+/g)
        .filter(part => !/^\w+:/.test(part))
        .join(' ')
    return { find, replace: m[2] }
}

export function queryWithReplacementText(query: string, replace: string): string {
    return `${query.slice(0, query.indexOf(' replace:'))} replace:${quoteIfNeeded(replace)}`
}