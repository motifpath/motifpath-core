-- create "course_enrollments" table
CREATE TABLE "course_enrollments" ("id" uuid NOT NULL, "student_id" uuid NOT NULL, "course_id" uuid NOT NULL, "course_title" character varying NOT NULL, "course_thumbnail_url" character varying NULL, "course_version_number" bigint NOT NULL, "status" character varying NOT NULL DEFAULT 'active', "active_checkpoint_student_path_id" uuid NULL, "active_checkpoint_position" bigint NULL, "enrolled_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "student_path_items" table
CREATE TABLE "student_path_items" ("id" uuid NOT NULL, "student_path_id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "content_node_version_id" uuid NOT NULL, "position" bigint NOT NULL, "section_label" character varying NULL, PRIMARY KEY ("id"));
-- create index "studentpathitem_student_path_id_position" to table: "student_path_items"
CREATE UNIQUE INDEX "studentpathitem_student_path_id_position" ON "student_path_items" ("student_path_id", "position");
-- create "student_paths" table
CREATE TABLE "student_paths" ("id" uuid NOT NULL, "student_id" uuid NOT NULL, "source_template_id" uuid NOT NULL, "title" character varying NOT NULL, "assigned_by" uuid NOT NULL, "assigned_at" timestamptz NOT NULL, "archived_at" timestamptz NULL, "source_course_enrollment_id" uuid NULL, "course_checkpoint_position" bigint NULL, "summary_snapshot" character varying NULL, "level_snapshot" character varying NULL, "thumbnail_url_snapshot" character varying NULL, "created_by_snapshot" uuid NULL, PRIMARY KEY ("id"));
-- create index "studentpath_student_id_source_template_id" to table: "student_paths"
CREATE UNIQUE INDEX "studentpath_student_id_source_template_id" ON "student_paths" ("student_id", "source_template_id") WHERE ((archived_at IS NULL) AND (source_course_enrollment_id IS NULL));
-- create "student_learning_states" table
CREATE TABLE "student_learning_states" ("id" uuid NOT NULL, "student_id" uuid NOT NULL, "current_course_enrollment_id" uuid NULL, "current_standalone_path_id" uuid NULL, PRIMARY KEY ("id"));
-- create index "student_learning_states_student_id_key" to table: "student_learning_states"
CREATE UNIQUE INDEX "student_learning_states_student_id_key" ON "student_learning_states" ("student_id");
-- create "learning_path_items" table
CREATE TABLE "learning_path_items" ("id" uuid NOT NULL, "learning_path_id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "position" bigint NOT NULL, "section_label" character varying NULL, PRIMARY KEY ("id"));
-- create index "learningpathitem_learning_path_id_position" to table: "learning_path_items"
CREATE UNIQUE INDEX "learningpathitem_learning_path_id_position" ON "learning_path_items" ("learning_path_id", "position");
-- create "expanded_contents" table
CREATE TABLE "expanded_contents" ("id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "content_type" character varying NOT NULL, "media_url" character varying NULL, "rich_content" text NULL, "diagram_ref" text NULL, "diagram_stack_ref" text NULL, "trigger_at_seconds" bigint NULL, "hide_at_seconds" bigint NULL, "trigger_at_paragraph" bigint NULL, "duration_ms" bigint NULL, "caption" character varying NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "expandedcontent_content_node_id" to table: "expanded_contents"
CREATE INDEX "expandedcontent_content_node_id" ON "expanded_contents" ("content_node_id");
-- create "challenges" table
CREATE TABLE "challenges" ("id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "subject_skill_id" uuid NULL, "subject_concept_id" uuid NULL, "pass_threshold" bigint NOT NULL, "time_threshold_ms" bigint NULL, "shuffle_exercises" boolean NOT NULL DEFAULT false, "shuffle_options" boolean NOT NULL DEFAULT false, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "challenge_content_node_id" to table: "challenges"
CREATE INDEX "challenge_content_node_id" ON "challenges" ("content_node_id");
-- create "course_version_checkpoints" table
CREATE TABLE "course_version_checkpoints" ("id" uuid NOT NULL, "course_version_id" uuid NOT NULL, "learning_path_id" uuid NOT NULL, "position" bigint NOT NULL, "effective_title" character varying NOT NULL, PRIMARY KEY ("id"));
-- create index "courseversioncheckpoint_course_version_id_position" to table: "course_version_checkpoints"
CREATE UNIQUE INDEX "courseversioncheckpoint_course_version_id_position" ON "course_version_checkpoints" ("course_version_id", "position");
-- create "content_node_versions" table
CREATE TABLE "content_node_versions" ("id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "version_number" bigint NOT NULL, "title" character varying NOT NULL, "content_type" character varying NOT NULL, "media_url" character varying NULL, "rich_content" text NULL, "classification_snapshot" text NULL, "languages_snapshot" text NULL, "instrument_ids_snapshot" jsonb NULL, "thumbnail_url_snapshot" character varying NULL, "published_by" uuid NOT NULL, "published_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "contentnodeversion_content_node_id_version_number" to table: "content_node_versions"
CREATE UNIQUE INDEX "contentnodeversion_content_node_id_version_number" ON "content_node_versions" ("content_node_id", "version_number");
-- create "course_versions" table
CREATE TABLE "course_versions" ("id" uuid NOT NULL, "course_id" uuid NOT NULL, "version_number" bigint NOT NULL, "title_snapshot" character varying NOT NULL, "summary_snapshot" character varying NOT NULL, "level_snapshot" character varying NOT NULL, "language_snapshot" character varying NOT NULL DEFAULT 'en', "instrument_ids_snapshot" jsonb NULL, "thumbnail_url_snapshot" character varying NULL, "available_for_new_enrollments" boolean NOT NULL DEFAULT true, "published_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "courseversion_course_id_version_number" to table: "course_versions"
CREATE UNIQUE INDEX "courseversion_course_id_version_number" ON "course_versions" ("course_id", "version_number");
-- create "course_checkpoints" table
CREATE TABLE "course_checkpoints" ("id" uuid NOT NULL, "course_id" uuid NOT NULL, "learning_path_id" uuid NOT NULL, "position" bigint NOT NULL, "title" character varying NULL, PRIMARY KEY ("id"));
-- create index "coursecheckpoint_course_id_position" to table: "course_checkpoints"
CREATE UNIQUE INDEX "coursecheckpoint_course_id_position" ON "course_checkpoints" ("course_id", "position");
-- create "exercises" table
CREATE TABLE "exercises" ("id" uuid NOT NULL, "title" character varying NOT NULL, "prompt" text NOT NULL, "exercise_type" character varying NOT NULL, "image_url" character varying NULL, "audio_url" character varying NULL, "estimated_duration_seconds" bigint NULL, "remediation_targets" text NULL, "diagram_ref" text NULL, "diagram_stack_ref" text NULL, "created_by" uuid NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "challenge_exercises" table
CREATE TABLE "challenge_exercises" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "challenge_id" uuid NOT NULL, "exercise_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "challenge_exercises_challenges_challenge" FOREIGN KEY ("challenge_id") REFERENCES "challenges" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "challenge_exercises_exercises_exercise" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "challengeexercise_challenge_id_exercise_id" to table: "challenge_exercises"
CREATE UNIQUE INDEX "challengeexercise_challenge_id_exercise_id" ON "challenge_exercises" ("challenge_id", "exercise_id");
-- create "content_nodes" table
CREATE TABLE "content_nodes" ("id" uuid NOT NULL, "teacher_id" uuid NOT NULL, "title" character varying NOT NULL, "content_type" character varying NOT NULL, "media_url" character varying NULL, "rich_content" text NULL, "difficulty_level" character varying NOT NULL, "review_state" character varying NOT NULL DEFAULT 'pending', "thumbnail_url" character varying NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "knowledge_nodes" table
CREATE TABLE "knowledge_nodes" ("id" uuid NOT NULL, "kind" character varying NOT NULL, "key" character varying NOT NULL, "names" jsonb NOT NULL, "descriptions" jsonb NULL, "parent_id" uuid NULL, PRIMARY KEY ("id"), CONSTRAINT "knowledge_nodes_knowledge_nodes_parent" FOREIGN KEY ("parent_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "knowledge_nodes_key_key" to table: "knowledge_nodes"
CREATE UNIQUE INDEX "knowledge_nodes_key_key" ON "knowledge_nodes" ("key");
-- create "content_node_concepts" table
CREATE TABLE "content_node_concepts" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "content_node_id" uuid NOT NULL, "concept_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "content_node_concepts_content_nodes_content_node" FOREIGN KEY ("content_node_id") REFERENCES "content_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "content_node_concepts_knowledge_nodes_concept" FOREIGN KEY ("concept_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "contentnodeconcept_content_node_id_concept_id" to table: "content_node_concepts"
CREATE UNIQUE INDEX "contentnodeconcept_content_node_id_concept_id" ON "content_node_concepts" ("content_node_id", "concept_id");
-- create "content_node_exercises" table
CREATE TABLE "content_node_exercises" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "content_node_id" uuid NOT NULL, "exercise_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "content_node_exercises_content_nodes_content_node" FOREIGN KEY ("content_node_id") REFERENCES "content_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "content_node_exercises_exercises_exercise" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "contentnodeexercise_content_node_id_exercise_id" to table: "content_node_exercises"
CREATE UNIQUE INDEX "contentnodeexercise_content_node_id_exercise_id" ON "content_node_exercises" ("content_node_id", "exercise_id");
-- create "voices" table
CREATE TABLE "voices" ("id" character varying NOT NULL, "names" jsonb NOT NULL, "family" character varying NOT NULL, "pitches" jsonb NOT NULL, "attribution" character varying NOT NULL, PRIMARY KEY ("id"));
-- create "instruments" table
CREATE TABLE "instruments" ("id" uuid NOT NULL, "names" jsonb NOT NULL, "family" character varying NOT NULL, "string_count" bigint NULL, "tuning" jsonb NULL, "key_range_lowest" character varying NULL, "key_range_highest" character varying NULL, "icon" character varying NOT NULL DEFAULT 'fretted', "default_voice_id" character varying NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "instruments_voices_default_voice" FOREIGN KEY ("default_voice_id") REFERENCES "voices" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create "content_node_instruments" table
CREATE TABLE "content_node_instruments" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "content_node_id" uuid NOT NULL, "instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "content_node_instruments_content_nodes_content_node" FOREIGN KEY ("content_node_id") REFERENCES "content_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "content_node_instruments_instruments_instrument" FOREIGN KEY ("instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "contentnodeinstrument_content_node_id_instrument_id" to table: "content_node_instruments"
CREATE UNIQUE INDEX "contentnodeinstrument_content_node_id_instrument_id" ON "content_node_instruments" ("content_node_id", "instrument_id");
-- create "languages" table
CREATE TABLE "languages" ("id" uuid NOT NULL, "code" character varying NOT NULL, "name" character varying NOT NULL, PRIMARY KEY ("id"));
-- create index "languages_code_key" to table: "languages"
CREATE UNIQUE INDEX "languages_code_key" ON "languages" ("code");
-- create "content_node_languages" table
CREATE TABLE "content_node_languages" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "content_node_id" uuid NOT NULL, "language_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "content_node_languages_content_nodes_content_node" FOREIGN KEY ("content_node_id") REFERENCES "content_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "content_node_languages_languages_language" FOREIGN KEY ("language_id") REFERENCES "languages" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "contentnodelanguage_content_node_id_language_id" to table: "content_node_languages"
CREATE UNIQUE INDEX "contentnodelanguage_content_node_id_language_id" ON "content_node_languages" ("content_node_id", "language_id");
-- create "content_node_skills" table
CREATE TABLE "content_node_skills" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "content_node_id" uuid NOT NULL, "skill_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "content_node_skills_content_nodes_content_node" FOREIGN KEY ("content_node_id") REFERENCES "content_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "content_node_skills_knowledge_nodes_skill" FOREIGN KEY ("skill_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "contentnodeskill_content_node_id_skill_id" to table: "content_node_skills"
CREATE UNIQUE INDEX "contentnodeskill_content_node_id_skill_id" ON "content_node_skills" ("content_node_id", "skill_id");
-- create "courses" table
CREATE TABLE "courses" ("id" uuid NOT NULL, "title" character varying NOT NULL, "summary" character varying NOT NULL, "level" character varying NOT NULL, "language" character varying NOT NULL DEFAULT 'en', "status" character varying NOT NULL DEFAULT 'draft', "thumbnail_url" character varying NULL, "created_by" uuid NOT NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "course_instruments" table
CREATE TABLE "course_instruments" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "course_id" uuid NOT NULL, "instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "course_instruments_courses_course" FOREIGN KEY ("course_id") REFERENCES "courses" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "course_instruments_instruments_instrument" FOREIGN KEY ("instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "courseinstrument_course_id_instrument_id" to table: "course_instruments"
CREATE UNIQUE INDEX "courseinstrument_course_id_instrument_id" ON "course_instruments" ("course_id", "instrument_id");
-- create "diagrams" table
CREATE TABLE "diagrams" ("id" uuid NOT NULL, "names" jsonb NOT NULL, "kind" character varying NOT NULL, "created_by" uuid NOT NULL, "root_note" character varying NULL, "label_display" character varying NOT NULL DEFAULT 'interval', "color" character varying NULL, "mode" character varying NULL, "playbacks" jsonb NOT NULL, "default_playback_id" character varying NULL, "created_at" timestamptz NOT NULL, "instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "diagrams_instruments_instrument" FOREIGN KEY ("instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create "diagram_concepts" table
CREATE TABLE "diagram_concepts" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "diagram_id" uuid NOT NULL, "concept_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "diagram_concepts_diagrams_diagram" FOREIGN KEY ("diagram_id") REFERENCES "diagrams" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "diagram_concepts_knowledge_nodes_concept" FOREIGN KEY ("concept_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "diagramconcept_diagram_id_concept_id" to table: "diagram_concepts"
CREATE UNIQUE INDEX "diagramconcept_diagram_id_concept_id" ON "diagram_concepts" ("diagram_id", "concept_id");
-- create "diagram_instruments" table
CREATE TABLE "diagram_instruments" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "diagram_id" uuid NOT NULL, "instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "diagram_instruments_diagrams_diagram" FOREIGN KEY ("diagram_id") REFERENCES "diagrams" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "diagram_instruments_instruments_instrument" FOREIGN KEY ("instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "diagraminstrument_diagram_id_instrument_id" to table: "diagram_instruments"
CREATE UNIQUE INDEX "diagraminstrument_diagram_id_instrument_id" ON "diagram_instruments" ("diagram_id", "instrument_id");
-- create "diagram_regions" table
CREATE TABLE "diagram_regions" ("id" uuid NOT NULL, "ordinal" bigint NOT NULL, "fret_start" bigint NULL, "fret_end" bigint NULL, "string_start" bigint NULL, "string_end" bigint NULL, "key_start" character varying NULL, "key_end" character varying NULL, "description" jsonb NOT NULL, "color" character varying NULL, "diagram_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "diagram_regions_diagrams_regions" FOREIGN KEY ("diagram_id") REFERENCES "diagrams" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "diagramregion_diagram_id_ordinal" to table: "diagram_regions"
CREATE UNIQUE INDEX "diagramregion_diagram_id_ordinal" ON "diagram_regions" ("diagram_id", "ordinal");
-- create "diagram_skills" table
CREATE TABLE "diagram_skills" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "diagram_id" uuid NOT NULL, "skill_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "diagram_skills_diagrams_diagram" FOREIGN KEY ("diagram_id") REFERENCES "diagrams" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "diagram_skills_knowledge_nodes_skill" FOREIGN KEY ("skill_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "diagramskill_diagram_id_skill_id" to table: "diagram_skills"
CREATE UNIQUE INDEX "diagramskill_diagram_id_skill_id" ON "diagram_skills" ("diagram_id", "skill_id");
-- create "drill_templates" table
CREATE TABLE "drill_templates" ("id" uuid NOT NULL, "key" character varying NOT NULL, "item_kind" character varying NOT NULL, "response_type" character varying NOT NULL, "timed" boolean NOT NULL, "names" jsonb NOT NULL, PRIMARY KEY ("id"));
-- create index "drill_templates_key_key" to table: "drill_templates"
CREATE UNIQUE INDEX "drill_templates_key_key" ON "drill_templates" ("key");
-- create "drill_thresholds" table
CREATE TABLE "drill_thresholds" ("id" uuid NOT NULL, "version" bigint NOT NULL, "effective_from" timestamptz NOT NULL, "fluent_net_ms" bigint NOT NULL, "source" character varying NOT NULL, "sessions" bigint NOT NULL, "students" bigint NOT NULL, "template_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "drill_thresholds_drill_templates_template" FOREIGN KEY ("template_id") REFERENCES "drill_templates" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "drillthreshold_template_id_version" to table: "drill_thresholds"
CREATE UNIQUE INDEX "drillthreshold_template_id_version" ON "drill_thresholds" ("template_id", "version");
-- create "exercise_concepts" table
CREATE TABLE "exercise_concepts" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "exercise_id" uuid NOT NULL, "concept_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "exercise_concepts_exercises_exercise" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "exercise_concepts_knowledge_nodes_concept" FOREIGN KEY ("concept_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "exerciseconcept_exercise_id_concept_id" to table: "exercise_concepts"
CREATE UNIQUE INDEX "exerciseconcept_exercise_id_concept_id" ON "exercise_concepts" ("exercise_id", "concept_id");
-- create "exercise_instruments" table
CREATE TABLE "exercise_instruments" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "exercise_id" uuid NOT NULL, "instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "exercise_instruments_exercises_exercise" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "exercise_instruments_instruments_instrument" FOREIGN KEY ("instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "exerciseinstrument_exercise_id_instrument_id" to table: "exercise_instruments"
CREATE UNIQUE INDEX "exerciseinstrument_exercise_id_instrument_id" ON "exercise_instruments" ("exercise_id", "instrument_id");
-- create "exercise_languages" table
CREATE TABLE "exercise_languages" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "exercise_id" uuid NOT NULL, "language_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "exercise_languages_exercises_exercise" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "exercise_languages_languages_language" FOREIGN KEY ("language_id") REFERENCES "languages" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "exerciselanguage_exercise_id_language_id" to table: "exercise_languages"
CREATE UNIQUE INDEX "exerciselanguage_exercise_id_language_id" ON "exercise_languages" ("exercise_id", "language_id");
-- create "exercise_options" table
CREATE TABLE "exercise_options" ("id" uuid NOT NULL, "is_correct" boolean NOT NULL, "label" character varying NULL, "image_url" character varying NULL, "audio_url" character varying NULL, "region_x" double precision NULL, "region_y" double precision NULL, "region_width" double precision NULL, "region_height" double precision NULL, "region_shape" character varying NULL, "diagram_ref" text NULL, "diagram_id" uuid NULL, "diagram_position_id" uuid NULL, "cell_string" bigint NULL, "cell_fret" bigint NULL, "exercise_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "exercise_options_exercises_options" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "exerciseoption_exercise_id" to table: "exercise_options"
CREATE INDEX "exerciseoption_exercise_id" ON "exercise_options" ("exercise_id");
-- create "exercise_skills" table
CREATE TABLE "exercise_skills" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "exercise_id" uuid NOT NULL, "skill_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "exercise_skills_exercises_exercise" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "exercise_skills_knowledge_nodes_skill" FOREIGN KEY ("skill_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "exerciseskill_exercise_id_skill_id" to table: "exercise_skills"
CREATE UNIQUE INDEX "exerciseskill_exercise_id_skill_id" ON "exercise_skills" ("exercise_id", "skill_id");
-- create "fretboard_cell_ranges" table
CREATE TABLE "fretboard_cell_ranges" ("id" uuid NOT NULL, "strings" jsonb NOT NULL, "from_fret" bigint NOT NULL, "to_fret" bigint NOT NULL, "skill_id" uuid NOT NULL, "layout_instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "fretboard_cell_ranges_instruments_layout_instrument" FOREIGN KEY ("layout_instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "fretboard_cell_ranges_knowledge_nodes_skill" FOREIGN KEY ("skill_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "fretboardcellrange_skill_id_layout_instrument_id" to table: "fretboard_cell_ranges"
CREATE UNIQUE INDEX "fretboardcellrange_skill_id_layout_instrument_id" ON "fretboard_cell_ranges" ("skill_id", "layout_instrument_id");
-- create "knowledge_edges" table
CREATE TABLE "knowledge_edges" ("id" uuid NOT NULL, "type" character varying NOT NULL, "level" character varying NULL, "from_id" uuid NOT NULL, "to_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "knowledge_edges_knowledge_nodes_from" FOREIGN KEY ("from_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "knowledge_edges_knowledge_nodes_to" FOREIGN KEY ("to_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "knowledgeedge_from_id_to_id_type" to table: "knowledge_edges"
CREATE UNIQUE INDEX "knowledgeedge_from_id_to_id_type" ON "knowledge_edges" ("from_id", "to_id", "type");
-- create index "knowledgeedge_to_id" to table: "knowledge_edges"
CREATE INDEX "knowledgeedge_to_id" ON "knowledge_edges" ("to_id");
-- create "knowledge_node_instruments" table
CREATE TABLE "knowledge_node_instruments" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "knowledge_node_id" uuid NOT NULL, "instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "knowledge_node_instruments_instruments_instrument" FOREIGN KEY ("instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "knowledge_node_instruments_knowledge_nodes_knowledge_node" FOREIGN KEY ("knowledge_node_id") REFERENCES "knowledge_nodes" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "knowledgenodeinstrument_knowledge_node_id_instrument_id" to table: "knowledge_node_instruments"
CREATE UNIQUE INDEX "knowledgenodeinstrument_knowledge_node_id_instrument_id" ON "knowledge_node_instruments" ("knowledge_node_id", "instrument_id");
-- create "learning_paths" table
CREATE TABLE "learning_paths" ("id" uuid NOT NULL, "teacher_id" uuid NOT NULL, "title" character varying NOT NULL, "summary" character varying NULL, "language" character varying NULL, "status" character varying NOT NULL DEFAULT 'draft', "level" character varying NULL, "updated_at" timestamptz NOT NULL, "thumbnail_url" character varying NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "learning_path_instruments" table
CREATE TABLE "learning_path_instruments" ("id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY, "linked_at" timestamptz NOT NULL, "learning_path_id" uuid NOT NULL, "instrument_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "learning_path_instruments_instruments_instrument" FOREIGN KEY ("instrument_id") REFERENCES "instruments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "learning_path_instruments_learning_paths_learning_path" FOREIGN KEY ("learning_path_id") REFERENCES "learning_paths" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "learningpathinstrument_learning_path_id_instrument_id" to table: "learning_path_instruments"
CREATE UNIQUE INDEX "learningpathinstrument_learning_path_id_instrument_id" ON "learning_path_instruments" ("learning_path_id", "instrument_id");
-- create "positions" table
CREATE TABLE "positions" ("id" uuid NOT NULL, "ordinal" bigint NOT NULL, "interval" character varying NOT NULL, "note_name" character varying NOT NULL, "shape" character varying NOT NULL DEFAULT 'dot', "color" character varying NULL, "string_number" bigint NULL, "fret" bigint NULL, "key" character varying NULL, "custom_label" jsonb NULL, "note" jsonb NULL, "diagram_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "positions_diagrams_positions" FOREIGN KEY ("diagram_id") REFERENCES "diagrams" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "position_diagram_id_ordinal" to table: "positions"
CREATE UNIQUE INDEX "position_diagram_id_ordinal" ON "positions" ("diagram_id", "ordinal");
-- create "users" table
CREATE TABLE "users" ("id" uuid NOT NULL, "clerk_user_id" character varying NOT NULL, "role" character varying NOT NULL, "display_name" character varying NOT NULL, "registered_at" timestamptz NOT NULL, "locale_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "users_languages_locale" FOREIGN KEY ("locale_id") REFERENCES "languages" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "users_clerk_user_id_key" to table: "users"
CREATE UNIQUE INDEX "users_clerk_user_id_key" ON "users" ("clerk_user_id");
