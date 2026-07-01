import React, { useCallback, useEffect, useState, type ChangeEvent } from 'react';
import { InlineField, InlineFieldRow, InlineSwitch, Input, Select, TextArea } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';

import { DataSource } from '../datasource';
import {
  ConfluenceQuery,
  ConfluenceDataSourceOptions,
  ConfluenceQueryType,
  SpaceInfo,
  QUERY_TYPE_OPTIONS,
  SORT_OPTIONS,
  DEFAULT_QUERY,
} from '../types';

type Props = QueryEditorProps<DataSource, ConfluenceQuery, ConfluenceDataSourceOptions>;

const LABEL_WIDTH = 20;
const INPUT_WIDTH = 40;
const WIDE_WIDTH = 60;

function spaceOptions(spaces: SpaceInfo[]): Array<SelectableValue<string>> {
  return spaces.map((s) => ({ label: `${s.name} (${s.key})`, value: s.id, description: `id: ${s.id}` }));
}

function errMessage(err: unknown, fallback: string): string {
  const e = err as { data?: { error?: string }; message?: string };
  return e?.data?.error ?? e?.message ?? fallback;
}

export function QueryEditor({ query, onChange, onRunQuery, datasource }: Props) {
  const { spaceId, cql, sort, fields, limit } = query;
  const queryType: ConfluenceQueryType = query.queryType ?? DEFAULT_QUERY.queryType!;

  const isContentType = queryType === 'pages' || queryType === 'blogposts';
  const isSearchType = queryType === 'search';
  const isCountType = queryType === 'count';
  // Space scoping applies to pages, blog posts and the default (non-CQL) count.
  const showSpace = isContentType || isCountType;
  // CQL applies to search, and optionally narrows a count query.
  const showCQL = isSearchType || isCountType;

  const [spaces, setSpaces] = useState<Array<SelectableValue<string>>>([]);
  const [spacesLoading, setSpacesLoading] = useState(false);
  const [spacesError, setSpacesError] = useState<string | undefined>();

  useEffect(() => {
    if (!showSpace) {
      return;
    }
    let cancelled = false;
    setSpacesLoading(true);
    setSpacesError(undefined);
    datasource
      .getSpaces()
      .then((items) => {
        if (!cancelled) {
          setSpaces(spaceOptions(items));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setSpaces([]);
          setSpacesError(errMessage(err, 'Failed to load spaces'));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setSpacesLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [datasource, showSpace]);

  const update = useCallback((patch: Partial<ConfluenceQuery>) => onChange({ ...query, ...patch }), [onChange, query]);
  const updateAndRun = useCallback(
    (patch: Partial<ConfluenceQuery>) => {
      onChange({ ...query, ...patch });
      onRunQuery();
    },
    [onChange, onRunQuery, query]
  );

  const onQueryTypeChange = (value: ConfluenceQueryType) => updateAndRun({ queryType: value });

  const selectedSpace: SelectableValue<string> | null = spaceId
    ? spaces.find((o) => o.value === spaceId) ?? { label: spaceId, value: spaceId }
    : null;
  const selectedSort: SelectableValue<string> | null = sort ? SORT_OPTIONS.find((o) => o.value === sort) ?? { label: sort, value: sort } : null;

  return (
    <div>
      <InlineField
        label="Query type"
        labelWidth={LABEL_WIDTH}
        tooltip="Pages/Blog posts list content via the v2 API; Search runs a CQL query; Count returns the number of matching items."
      >
        <Select<string>
          width={INPUT_WIDTH}
          options={QUERY_TYPE_OPTIONS}
          value={queryType}
          onChange={(v) => {
            if (v?.value) {
              onQueryTypeChange(v.value as ConfluenceQueryType);
            }
          }}
          placeholder="Select query type"
        />
      </InlineField>

      {showSpace && (
        <InlineFieldRow>
          <InlineField
            label="Space"
            labelWidth={LABEL_WIDTH}
            tooltip="Restrict to a single space. Leave empty to query across all spaces."
            error={spacesError}
            invalid={!!spacesError}
          >
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              isLoading={spacesLoading}
              options={spaces}
              value={selectedSpace}
              onChange={(v) => updateAndRun({ spaceId: v?.value ?? '' })}
              allowCustomValue
              placeholder="All spaces"
              noOptionsMessage="No spaces found"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {isContentType && (
        <InlineFieldRow>
          <InlineField label="Sort" labelWidth={LABEL_WIDTH} tooltip="Sort order for the returned content.">
            <Select<string>
              width={INPUT_WIDTH}
              isClearable
              options={SORT_OPTIONS}
              value={selectedSort}
              onChange={(v) => updateAndRun({ sort: v?.value ?? '' })}
              allowCustomValue
              placeholder="Default order"
            />
          </InlineField>
        </InlineFieldRow>
      )}

      {showCQL && (
        <div className="gf-form" style={{ alignItems: 'flex-start' }}>
          <InlineField
            label="CQL"
            labelWidth={LABEL_WIDTH}
            tooltip={
              isCountType
                ? 'Optional CQL. When set, the count is over the CQL search results instead of pages in the space.'
                : 'Confluence Query Language, e.g. type = page AND space = "ENG" AND text ~ "release".'
            }
            grow
          >
            <TextArea
              rows={3}
              value={cql ?? ''}
              placeholder='type = page AND text ~ "release notes"'
              onChange={(e: React.FormEvent<HTMLTextAreaElement>) => update({ cql: e.currentTarget.value })}
              onBlur={onRunQuery}
            />
          </InlineField>
        </div>
      )}

      {!isCountType && (
        <>
          <InlineFieldRow>
            <InlineField
              label="Fields"
              labelWidth={LABEL_WIDTH}
              tooltip="Comma-separated list of columns to return. Leave empty to return all flattened columns."
            >
              <Input
                width={WIDE_WIDTH}
                value={fields ?? ''}
                placeholder="id,title,createdAt,webui"
                onChange={(e: ChangeEvent<HTMLInputElement>) => update({ fields: e.target.value })}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>

          <InlineFieldRow>
            <InlineField
              label="Limit"
              labelWidth={LABEL_WIDTH}
              tooltip="Maximum number of records. 0 returns all (auto-paginated)."
            >
              <Input
                width={20}
                type="number"
                min={0}
                value={limit ?? 0}
                onChange={(e: ChangeEvent<HTMLInputElement>) => {
                  const n = parseInt(e.target.value, 10);
                  update({ limit: isNaN(n) ? 0 : n });
                }}
                onBlur={onRunQuery}
              />
            </InlineField>
          </InlineFieldRow>
        </>
      )}
      <InlineFieldRow>
        <InlineField
          label="Hide system fields"
          labelWidth={LABEL_WIDTH}
          tooltip="Hide metadata-style columns (id, created_at/updated_at, underscore-prefixed names, etc.) from the returned frame."
        >
          <InlineSwitch
            value={!!query.hideSystemFields}
            onChange={(e) => {
              update({ hideSystemFields: e.currentTarget.checked });
              onRunQuery();
            }}
          />
        </InlineField>
      </InlineFieldRow>

    </div>
  );
}
