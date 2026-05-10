


CREATE TABLE tbl_application (
                                 user_id VARCHAR(100) PRIMARY KEY,
                                 first_name_th VARCHAR(255),
                                 first_name_en VARCHAR(255),
                                 mid_name_th VARCHAR(255),
                                 mid_name_en VARCHAR(255),
                                 last_name_th VARCHAR(255),
                                 last_name_en VARCHAR(255),
                                 phone VARCHAR(20) ,
                                 user_id_type VARCHAR(50),
                                 email VARCHAR(100) ,
                                 nationality VARCHAR(50),
                                 occupation VARCHAR(100),
                                 request_ref VARCHAR(100),
                                 birth_date DATE,
                                 gender CHAR(1),
                                 tax_id VARCHAR(20),
                                 second_email VARCHAR(100),
                                 occupation_other_desc VARCHAR(255),
                                 is_active varchar(1) DEFAULT 'Y',

    -- Pattern Fields
                                 create_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                 create_by VARCHAR(50),
                                 update_by VARCHAR(50),
                                 update_at TIMESTAMP,
                                 is_delete VARCHAR(1) DEFAULT 'N'
);


CREATE TABLE tbl_user (
                          user_id VARCHAR(100) ,
                          first_name_th VARCHAR(255),
                          first_name_en VARCHAR(255),
                          mid_name_th VARCHAR(255),
                          mid_name_en VARCHAR(255),
                          last_name_th VARCHAR(255),
                          last_name_en VARCHAR(255),
                          phone VARCHAR(20) ,
                          user_id_type VARCHAR(50),
                          email VARCHAR(100),
                          nationality VARCHAR(50),
                          occupation VARCHAR(100),
                          request_ref VARCHAR(100),
                          birth_date DATE,
                          gender CHAR(1),
                          tax_id VARCHAR(20),
                          second_email VARCHAR(100),
                          occupation_other_desc VARCHAR(255),
                          is_active varchar(1) DEFAULT 'Y',
                          password VARCHAR(255),
                          branchCode VARCHAR(50),
                          appCode VARCHAR(50),
                          companyCode VARCHAR(50),
                          status VARCHAR(50),
                          account_name VARCHAR(100),
                          user_active_time TIMESTAMP,
                          external_id VARCHAR(100),
                          user_details JSON,
                          in_active BOOLEAN DEFAULT FALSE,

    -- Pattern Fields
                          create_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                          create_by VARCHAR(50),
                          update_by VARCHAR(50),
                          update_at TIMESTAMP,
                          is_delete VARCHAR(1) DEFAULT 'N',

                          PRIMARY KEY (user_id, appCode)

);
ALTER TABLE tbl_user
ALTER COLUMN is_active TYPE VARCHAR(1) USING (is_active::VARCHAR(1)),
ALTER COLUMN is_active SET DEFAULT 'N';


create INDEX idx_user_id_branch_company on tbl_user (user_id, branchCode, companyCode);
create INDEX idx_user_company_branch on tbl_user(companyCode, branchCode);
create INDEX idx_user_company on tbl_user(companyCode);




CREATE TABLE tbl_role (
                          id serial PRIMARY KEY, -- Make `id` the primary key
                          role_code VARCHAR(50) UNIQUE, -- Ensure `role_code` is unique
                          parent_role_id int REFERENCES tbl_role(id) ON DELETE SET NULL, -- Reference the primary key
                          role_name_th VARCHAR(100),
                          role_name_en VARCHAR(100),
                          role_desc_th VARCHAR(255),
                          role_desc_en VARCHAR(255),

                          create_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                          create_by VARCHAR(50),
                          update_by VARCHAR(50),
                          update_at TIMESTAMP,
                          is_delete VARCHAR(1) DEFAULT 'N'
);


CREATE TABLE tbl_user_role (
                               role_code VARCHAR(50) REFERENCES tbl_role(role_code) ON DELETE CASCADE,
                               user_id VARCHAR(100),
                               appCode VARCHAR(50),
                               FOREIGN KEY (user_id, appCode) REFERENCES tbl_user(user_id, appCode) ON DELETE CASCADE,
                               PRIMARY KEY (role_code, user_id, appCode),

                               create_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                               create_by VARCHAR(50),
                               update_by VARCHAR(50),
                               update_at TIMESTAMP,
                               is_delete VARCHAR(1) DEFAULT 'N'
);






CREATE TABLE tbl_object (
                            object_code VARCHAR(50) PRIMARY KEY,
                            object_name_th VARCHAR(100),
                            object_name_en VARCHAR(100),
                            object_desc_th VARCHAR(255),
                            object_desc_en VARCHAR(255),


                            create_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                            create_by VARCHAR(50),
                            update_by VARCHAR(50),
                            update_at TIMESTAMP,
                            is_delete VARCHAR(1) DEFAULT 'N'
);


CREATE TABLE tbl_role_object (
                                 role_code VARCHAR(50) REFERENCES tbl_role(role_code) ON DELETE CASCADE,
                                 object_code VARCHAR(50) REFERENCES tbl_object(object_code) ON DELETE CASCADE,


                                 PRIMARY KEY (role_code, object_code),


                                 create_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                                 create_by VARCHAR(50),
                                 update_by VARCHAR(50),
                                 update_at TIMESTAMP,
                                 is_delete VARCHAR(1) DEFAULT 'N'
);


CREATE TABLE tbl_app (
                         app_code VARCHAR(50) PRIMARY KEY,
                         app_name_th VARCHAR(255),
                         app_name_en VARCHAR(255),
                         app_picture BYTEA,
                         app_desc_th TEXT,
                         app_desc_en TEXT,
                         user_role VARCHAR(50),
                         user_approve CHAR(1) DEFAULT 'N',


                         create_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
                         create_by VARCHAR(50),
                         update_by VARCHAR(50),
                         update_at TIMESTAMP,
                         is_delete VARCHAR(1) DEFAULT 'N'
);


